package mail

import (
	"fmt"
	"log"
	"strings"
	"time"

	"mailnest-be/internal/crypto"
	"mailnest-be/internal/storage"
)

// TestConnection 只验证当前账号的 IMAP 配置是否可用，不会收取或保存邮件。
func (s *Service) TestConnection(userID, accountID int64) error {
	started := time.Now()
	log.Printf("邮箱连接测试开始 userID=%d accountID=%d", userID, accountID)
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return err
	}
	config, err := s.accountConfig(account)
	if err != nil {
		return err
	}
	if err := s.fetcher.TestConnection(config); err != nil {
		log.Printf("邮箱连接测试失败 userID=%d accountID=%d err=%v", userID, accountID, err)
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return err
	}
	log.Printf("邮箱连接测试成功 userID=%d accountID=%d duration=%s", userID, accountID, time.Since(started))
	return s.store.UpdateMailAccountSyncStatus(userID, accountID, "connection_ok", "")
}

// ListFolders 从 IMAP 服务端读取真实目录，供前端选择发件箱等特殊目录。
func (s *Service) ListFolders(userID, accountID int64) ([]FolderInfo, error) {
	started := time.Now()
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return nil, err
	}
	config, err := s.accountConfig(account)
	if err != nil {
		return nil, err
	}
	folders, err := s.fetcher.ListFolders(config)
	if err != nil {
		log.Printf("读取邮箱目录失败 userID=%d accountID=%d err=%v", userID, accountID, err)
		return nil, err
	}
	log.Printf("读取邮箱目录成功 userID=%d accountID=%d count=%d duration=%s", userID, accountID, len(folders), time.Since(started))
	return folders, nil
}

// accountConfig 将数据库账号配置转换为 IMAP 拉取配置；密码和 token 只在内存中解密，不写日志。
func (s *Service) accountConfig(account storage.MailAccount) (AccountConfig, error) {
	password := ""
	accessToken := ""
	if account.AuthType == "oauth2" {
		token, err := s.oauthAccessToken(account)
		if err != nil {
			return AccountConfig{}, err
		}
		accessToken = token
	} else if strings.TrimSpace(account.IMAPPasswordEncoded) != "" {
		decrypted, err := crypto.DecryptString(account.IMAPPasswordEncoded, s.credentialSecret)
		if err != nil {
			return AccountConfig{}, err
		}
		password = decrypted
	}
	return AccountConfig{
		Email:       account.Email,
		Host:        account.IMAPHost,
		Port:        account.IMAPPort,
		TLS:         account.IMAPTLS,
		Username:    account.IMAPUsername,
		Password:    password,
		AccessToken: accessToken,
		AuthType:    account.AuthType,
		Provider:    account.Provider,
		Folder:      "INBOX",
	}, nil
}

// smtpConfig 将数据库账号配置转换为 SMTP 发信配置；凭据解密后只在本次发信用途中使用。
func (s *Service) smtpConfig(account storage.MailAccount) (SMTPConfig, error) {
	if strings.TrimSpace(account.SMTPHost) == "" || account.SMTPPort <= 0 {
		return SMTPConfig{}, fmt.Errorf("请先在邮箱账号中配置 SMTP 发信服务器")
	}
	password := ""
	if strings.TrimSpace(account.SMTPPasswordEncoded) != "" {
		decrypted, err := crypto.DecryptString(account.SMTPPasswordEncoded, s.credentialSecret)
		if err != nil {
			return SMTPConfig{}, err
		}
		password = decrypted
	} else if account.AuthType != "oauth2" && strings.TrimSpace(account.IMAPPasswordEncoded) != "" {
		decrypted, err := crypto.DecryptString(account.IMAPPasswordEncoded, s.credentialSecret)
		if err != nil {
			return SMTPConfig{}, err
		}
		password = decrypted
	}
	username := strings.TrimSpace(account.SMTPUsername)
	if username == "" {
		username = account.Email
	}
	return SMTPConfig{
		Email:       account.Email,
		DisplayName: account.DisplayName,
		Host:        account.SMTPHost,
		Port:        account.SMTPPort,
		TLS:         account.SMTPTLS,
		StartTLS:    account.SMTPStartTLS,
		Username:    username,
		Password:    password,
	}, nil
}

func normalizeSentFolder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Sent"
	}
	return value
}

func normalizeFolderName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "INBOX"
	}
	return value
}

func (s *Service) oauthAccessToken(account storage.MailAccount) (string, error) {
	if account.OAuthAccessToken.Valid && (!account.OAuthExpiresAt.Valid || time.Until(account.OAuthExpiresAt.Time) > 2*time.Minute) {
		return crypto.DecryptString(account.OAuthAccessToken.String, s.credentialSecret)
	}
	if s.exchanger == nil || !account.OAuthRefreshToken.Valid {
		if account.OAuthAccessToken.Valid {
			return crypto.DecryptString(account.OAuthAccessToken.String, s.credentialSecret)
		}
		return "", fmt.Errorf("OAuth token 不存在，请重新授权")
	}
	refreshToken, err := crypto.DecryptString(account.OAuthRefreshToken.String, s.credentialSecret)
	if err != nil {
		return "", err
	}
	token, err := s.exchanger.Refresh(refreshToken)
	if err != nil {
		return "", err
	}
	encryptedAccess, err := crypto.EncryptString(token.AccessToken, s.credentialSecret)
	if err != nil {
		return "", err
	}
	encryptedRefresh := account.OAuthRefreshToken.String
	if strings.TrimSpace(token.RefreshToken) != "" {
		encryptedRefresh, err = crypto.EncryptString(token.RefreshToken, s.credentialSecret)
		if err != nil {
			return "", err
		}
	}
	if err := s.store.UpdateMailAccountOAuthTokens(account.UserID, account.ID, encryptedAccess, encryptedRefresh, token.ExpiresAt); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}
