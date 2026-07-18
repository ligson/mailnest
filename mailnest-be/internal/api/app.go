package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	netmail "net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mailnest-be/internal/auth"
	"mailnest-be/internal/config"
	"mailnest-be/internal/crypto"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"

	"golang.org/x/image/tiff"
)

type App struct {
	cfg         config.Config
	store       *storage.Store
	mailService *mail.Service
	oauth       *oauth.Service
}

type contextKey string

const userIDKey contextKey = "userID"

var cidReferencePattern = regexp.MustCompile(`(?i)cid:(?:<[^>]+>|[^"'\s>]+)`)

const inlineAttachmentURLTTL = time.Hour

const maxComposeAttachmentCount = 20
const maxComposeAttachmentBytes = 25 << 20

func NewApp(cfg config.Config) (*App, error) {
	return NewAppWithFetcher(cfg, nil)
}

func NewAppWithFetcher(cfg config.Config, fetcher mail.Fetcher) (*App, error) {
	return NewAppWithDependencies(cfg, fetcher, oauth.NewMicrosoftExchanger(cfg.OAuth.Microsoft))
}

func NewAppWithDependencies(cfg config.Config, fetcher mail.Fetcher, exchanger oauth.MicrosoftExchanger) (*App, error) {
	store, err := storage.Open(cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	if err := store.MarkStaleFullSyncsFailed(); err != nil {
		_ = store.Close()
		return nil, err
	}
	mailService := mail.NewService(store, fetcher, exchanger, cfg.App.DataDir, cfg.App.CredentialSecret)

	return &App{
		cfg:         cfg,
		store:       store,
		mailService: mailService,
		oauth:       oauth.NewService(store, exchanger, cfg.App.CredentialSecret, cfg.OAuth.Microsoft.RedirectURL),
	}, nil
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", a.handleHealth)
	mux.HandleFunc("POST /api/v1/auth/register", a.handleRegister)
	mux.HandleFunc("POST /api/v1/auth/login", a.handleLogin)
	mux.Handle("GET /api/v1/auth/me", a.authMiddleware(http.HandlerFunc(a.handleMe)))
	mux.Handle("POST /api/v1/auth/logout", a.authMiddleware(http.HandlerFunc(a.handleLogout)))
	mux.Handle("POST /api/v1/auth/change-password", a.authMiddleware(http.HandlerFunc(a.handleChangePassword)))
	mux.Handle("GET /api/v1/profile", a.authMiddleware(http.HandlerFunc(a.handleProfile)))
	mux.Handle("PUT /api/v1/profile", a.authMiddleware(http.HandlerFunc(a.handleUpdateProfile)))
	mux.Handle("POST /api/v1/profile/avatar", a.authMiddleware(http.HandlerFunc(a.handleUploadProfileAvatar)))
	mux.Handle("GET /api/v1/profile/avatar/content", a.authMiddleware(http.HandlerFunc(a.handleProfileAvatarContent)))
	mux.Handle("GET /api/v1/mail-accounts", a.authMiddleware(http.HandlerFunc(a.handleListMailAccounts)))
	mux.Handle("POST /api/v1/mail-accounts", a.authMiddleware(http.HandlerFunc(a.handleCreateMailAccount)))
	mux.Handle("PUT /api/v1/mail-accounts/{id}", a.authMiddleware(http.HandlerFunc(a.handleUpdateMailAccount)))
	mux.Handle("DELETE /api/v1/mail-accounts/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteMailAccount)))
	mux.Handle("GET /api/v1/mail-accounts/{id}/folders", a.authMiddleware(http.HandlerFunc(a.handleListMailAccountFolders)))
	mux.Handle("POST /api/v1/mail-accounts/{id}/test-connection", a.authMiddleware(http.HandlerFunc(a.handleTestMailAccountConnection)))
	mux.Handle("POST /api/v1/mail-accounts/{id}/sync", a.authMiddleware(http.HandlerFunc(a.handleSyncMailAccount)))
	mux.Handle("POST /api/v1/mail-accounts/{id}/full-sync/start", a.authMiddleware(http.HandlerFunc(a.handleStartFullSyncMailAccount)))
	mux.Handle("POST /api/v1/mail-accounts/{id}/full-sync/stop", a.authMiddleware(http.HandlerFunc(a.handleStopFullSyncMailAccount)))
	mux.Handle("GET /api/v1/mail-accounts/{id}/sync-status", a.authMiddleware(http.HandlerFunc(a.handleMailAccountSyncStatus)))
	mux.Handle("GET /api/v1/messages", a.authMiddleware(http.HandlerFunc(a.handleListMessages)))
	mux.Handle("POST /api/v1/messages/send", a.authMiddleware(http.HandlerFunc(a.handleSendMessage)))
	mux.Handle("GET /api/v1/messages/{id}", a.authMiddleware(http.HandlerFunc(a.handleMessageDetail)))
	mux.Handle("POST /api/v1/messages/{id}/folder", a.authMiddleware(http.HandlerFunc(a.handleAssignMessageFolder)))
	mux.Handle("GET /api/v1/messages/{id}/attachments/{attachmentId}/content", a.authMiddleware(http.HandlerFunc(a.handleAttachmentContent)))
	mux.HandleFunc("GET /api/v1/messages/{id}/attachments/{attachmentId}/inline-content", a.handleInlineAttachmentContent)
	mux.Handle("GET /api/v1/contacts", a.authMiddleware(http.HandlerFunc(a.handleListContacts)))
	mux.Handle("POST /api/v1/contacts", a.authMiddleware(http.HandlerFunc(a.handleCreateContact)))
	mux.Handle("PUT /api/v1/contacts/{id}", a.authMiddleware(http.HandlerFunc(a.handleUpdateContact)))
	mux.Handle("DELETE /api/v1/contacts/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteContact)))
	mux.Handle("GET /api/v1/mail-folders", a.authMiddleware(http.HandlerFunc(a.handleListMailFolders)))
	mux.Handle("POST /api/v1/mail-folders", a.authMiddleware(http.HandlerFunc(a.handleCreateMailFolder)))
	mux.Handle("DELETE /api/v1/mail-folders/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteMailFolder)))
	mux.Handle("GET /api/v1/mail-rules", a.authMiddleware(http.HandlerFunc(a.handleListMailRules)))
	mux.Handle("POST /api/v1/mail-rules", a.authMiddleware(http.HandlerFunc(a.handleCreateMailRule)))
	mux.Handle("PUT /api/v1/mail-rules/{id}", a.authMiddleware(http.HandlerFunc(a.handleUpdateMailRule)))
	mux.Handle("DELETE /api/v1/mail-rules/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteMailRule)))
	mux.Handle("POST /api/v1/mail-rules/apply", a.authMiddleware(http.HandlerFunc(a.handleApplyMailRules)))
	mux.Handle("POST /api/v1/oauth/microsoft/start", a.authMiddleware(http.HandlerFunc(a.handleMicrosoftOAuthStart)))
	mux.Handle("POST /api/v1/oauth/microsoft/complete", a.authMiddleware(http.HandlerFunc(a.handleMicrosoftOAuthComplete)))
	return mux
}

func (a *App) StartBackgroundTasks(ctx context.Context) {
	go func() {
		if err := a.mailService.RepairStoredParsedMessages(); err != nil {
			log.Printf("repair parsed mail content: %v", err)
		}
	}()
	a.mailService.StartAutoSyncScheduler(ctx, mail.AutoSyncOptions{})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	response.OK(w, "服务正常", map[string]any{"status": "ok"})
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type updateProfileRequest struct {
	Nickname string `json:"nickname"`
	Bio      string `json:"bio"`
}

type createMailAccountRequest struct {
	DisplayName          string `json:"displayName"`
	Email                string `json:"email"`
	IMAPHost             string `json:"imapHost"`
	IMAPPort             int    `json:"imapPort"`
	IMAPTLS              bool   `json:"imapTls"`
	IMAPUsername         string `json:"imapUsername"`
	IMAPPassword         string `json:"imapPassword"`
	SMTPHost             string `json:"smtpHost"`
	SMTPPort             int    `json:"smtpPort"`
	SMTPTLS              bool   `json:"smtpTls"`
	SMTPStartTLS         bool   `json:"smtpStartTls"`
	SMTPUsername         string `json:"smtpUsername"`
	SMTPPassword         string `json:"smtpPassword"`
	SMTPUseIMAPPassword  bool   `json:"smtpUseImapPassword"`
	SentFolder           string `json:"sentFolder"`
	SignatureHTML        string `json:"signatureHtml"`
	PollIntervalMinutes  int    `json:"pollIntervalMinutes"`
	Enabled              bool   `json:"enabled"`
	CleanupEnabled       bool   `json:"cleanupEnabled"`
	CleanupRetentionDays int    `json:"cleanupRetentionDays"`
}

type completeMicrosoftOAuthRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type createMailFolderRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
}

type contactRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Nickname    string `json:"nickname"`
	Phone       string `json:"phone"`
	Company     string `json:"company"`
	Notes       string `json:"notes"`
}

type assignMessageFolderRequest struct {
	FolderID string `json:"folderId"`
}

type sendMessageRequest struct {
	AccountID string   `json:"accountId"`
	To        []string `json:"to"`
	CC        []string `json:"cc"`
	BCC       []string `json:"bcc"`
	Subject   string   `json:"subject"`
	TextBody  string   `json:"textBody"`
	HTMLBody  string   `json:"htmlBody"`
}

type createMailRuleRequest struct {
	Name           string                    `json:"name"`
	Enabled        bool                      `json:"enabled"`
	MatchMode      string                    `json:"matchMode"`
	TargetFolderID string                    `json:"targetFolderId"`
	SortOrder      int                       `json:"sortOrder"`
	Conditions     []createMailRuleCondition `json:"conditions"`
}

type createMailRuleCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type applyMailRulesRequest struct {
	Scope string `json:"scope"`
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.App.AllowRegistration {
		response.Error(w, http.StatusForbidden, "当前系统未开放注册")
		return
	}

	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" || req.Email == "" || len(req.Password) < 8 {
		response.Error(w, http.StatusBadRequest, "用户名、邮箱不能为空，密码至少 8 位")
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "密码处理失败")
		return
	}

	user, err := a.store.CreateUser(req.Username, req.Email, passwordHash)
	if err != nil {
		response.Error(w, http.StatusConflict, "用户名或邮箱已存在")
		return
	}

	token, err := a.issueToken(user.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "登录凭据生成失败")
		return
	}

	response.Created(w, "注册成功", map[string]any{
		"user":  userPayload(user),
		"token": token,
	})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	user, err := a.store.FindUserByAccount(strings.TrimSpace(req.Account))
	if errors.Is(err, storage.ErrNotFound) || !auth.CheckPassword(user.PasswordHash, req.Password) {
		response.Error(w, http.StatusUnauthorized, "账号或密码错误")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "登录失败")
		return
	}

	token, err := a.issueToken(user.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "登录凭据生成失败")
		return
	}

	response.OK(w, "登录成功", map[string]any{
		"user":  userPayload(user),
		"token": token,
	})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(int64)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	user, err := a.store.FindUserByID(userID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取当前用户失败")
		return
	}

	response.OK(w, "获取成功", userPayload(user))
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	response.OK(w, "退出成功", nil)
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if strings.TrimSpace(req.CurrentPassword) == "" || len(req.NewPassword) < 8 {
		response.Error(w, http.StatusBadRequest, "当前密码不能为空，新密码至少 8 位")
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		response.Error(w, http.StatusBadRequest, "两次输入的新密码不一致")
		return
	}

	user, err := a.store.FindUserByID(userID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取当前用户失败")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		response.Error(w, http.StatusBadRequest, "当前密码错误")
		return
	}
	if auth.CheckPassword(user.PasswordHash, req.NewPassword) {
		response.Error(w, http.StatusBadRequest, "新密码不能与当前密码相同")
		return
	}

	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "密码处理失败")
		return
	}
	if err := a.store.UpdateUserPasswordHash(userID, passwordHash); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "密码修改失败")
		return
	}

	response.OK(w, "密码修改成功", nil)
}

func (a *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	user, err := a.store.FindUserByID(userID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取个人资料失败")
		return
	}

	response.OK(w, "获取成功", userPayload(user))
}

func (a *App) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	var req updateProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Bio = strings.TrimSpace(req.Bio)
	if len([]rune(req.Nickname)) > 40 {
		response.Error(w, http.StatusBadRequest, "昵称不能超过 40 个字符")
		return
	}
	if len([]rune(req.Bio)) > 200 {
		response.Error(w, http.StatusBadRequest, "个人描述不能超过 200 个字符")
		return
	}

	user, err := a.store.UpdateUserProfile(userID, req.Nickname, req.Bio)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "保存个人资料失败")
		return
	}

	response.OK(w, "保存成功", userPayload(user))
}

func (a *App) handleUploadProfileAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		response.Error(w, http.StatusBadRequest, "头像文件不能超过 2MB")
		return
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "请选择头像文件")
		return
	}
	defer file.Close()

	avatarPath, err := a.saveProfileAvatar(userID, file, header)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := a.store.UpdateUserAvatarPath(userID, avatarPath)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "保存头像失败")
		return
	}

	response.OK(w, "头像已更新", userPayload(user))
}

func (a *App) handleProfileAvatarContent(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	user, err := a.store.FindUserByID(userID)
	if errors.Is(err, storage.ErrNotFound) || !user.AvatarPath.Valid {
		response.Error(w, http.StatusNotFound, "头像不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取头像失败")
		return
	}
	http.ServeFile(w, r, user.AvatarPath.String)
}

func (a *App) saveProfileAvatar(userID int64, file multipart.File, header *multipart.FileHeader) (string, error) {
	extension := strings.ToLower(filepath.Ext(header.Filename))
	contentType := header.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "image/png"):
		extension = ".png"
	case strings.HasPrefix(contentType, "image/jpeg"):
		extension = ".jpg"
	case strings.HasPrefix(contentType, "image/webp"):
		extension = ".webp"
	case strings.HasPrefix(contentType, "image/gif"):
		extension = ".gif"
	}
	switch extension {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
	default:
		return "", fmt.Errorf("头像仅支持 PNG、JPG、WEBP 或 GIF")
	}

	dir := filepath.Join(a.cfg.App.DataDir, "users", fmt.Sprint(userID), "profile")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建头像目录失败")
	}
	path := filepath.Join(dir, "avatar"+extension)
	output, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("保存头像失败")
	}
	defer output.Close()
	if _, err := output.ReadFrom(file); err != nil {
		return "", fmt.Errorf("保存头像失败")
	}
	return path, nil
}

func (a *App) handleListMailAccounts(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	accounts, err := a.store.ListMailAccounts(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮箱账号失败")
		return
	}

	items := make([]map[string]any, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, mailAccountPayload(account))
	}

	response.OK(w, "获取成功", map[string]any{"items": items})
}

func (a *App) handleCreateMailAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	var req createMailAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)
	req.IMAPHost = strings.TrimSpace(req.IMAPHost)
	req.IMAPUsername = strings.TrimSpace(req.IMAPUsername)
	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SentFolder = normalizeSentFolder(req.SentFolder)
	req.SignatureHTML = strings.TrimSpace(req.SignatureHTML)
	if req.DisplayName == "" || req.Email == "" || req.IMAPHost == "" || req.IMAPUsername == "" || req.IMAPPassword == "" || req.IMAPPort <= 0 {
		response.Error(w, http.StatusBadRequest, "邮箱账号配置不完整")
		return
	}
	if req.SMTPHost != "" && req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}
	if req.PollIntervalMinutes <= 0 {
		req.PollIntervalMinutes = 10
	}
	if req.CleanupRetentionDays <= 0 {
		req.CleanupRetentionDays = 90
	}
	if len([]rune(req.SignatureHTML)) > 10000 {
		response.Error(w, http.StatusBadRequest, "签名模板不能超过 10000 个字符")
		return
	}

	encryptedPassword, err := crypto.EncryptString(req.IMAPPassword, a.cfg.App.CredentialSecret)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "邮箱凭据加密失败")
		return
	}
	encryptedSMTPPassword := ""
	if strings.TrimSpace(req.SMTPPassword) != "" {
		encryptedSMTPPassword, err = crypto.EncryptString(req.SMTPPassword, a.cfg.App.CredentialSecret)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "SMTP 凭据加密失败")
			return
		}
	} else if req.SMTPUseIMAPPassword && req.SMTPHost != "" {
		encryptedSMTPPassword = encryptedPassword
	}

	account, err := a.store.CreateMailAccount(storage.MailAccount{
		UserID:               userID,
		DisplayName:          req.DisplayName,
		Email:                req.Email,
		IMAPHost:             req.IMAPHost,
		IMAPPort:             req.IMAPPort,
		IMAPTLS:              req.IMAPTLS,
		IMAPUsername:         req.IMAPUsername,
		IMAPPasswordEncoded:  encryptedPassword,
		SMTPHost:             req.SMTPHost,
		SMTPPort:             req.SMTPPort,
		SMTPTLS:              req.SMTPTLS,
		SMTPStartTLS:         req.SMTPStartTLS,
		SMTPUsername:         req.SMTPUsername,
		SMTPPasswordEncoded:  encryptedSMTPPassword,
		SentFolder:           req.SentFolder,
		SignatureHTML:        req.SignatureHTML,
		PollIntervalMinutes:  req.PollIntervalMinutes,
		Enabled:              req.Enabled,
		CleanupEnabled:       req.CleanupEnabled,
		CleanupRetentionDays: req.CleanupRetentionDays,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "创建邮箱账号失败")
		return
	}

	response.Created(w, "创建成功", mailAccountPayload(account))
}

func (a *App) handleUpdateMailAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮箱账号 ID 格式错误")
		return
	}
	current, err := a.store.FindMailAccountByID(userID, accountID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮箱账号失败")
		return
	}

	var req createMailAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)
	req.IMAPHost = strings.TrimSpace(req.IMAPHost)
	req.IMAPUsername = strings.TrimSpace(req.IMAPUsername)
	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SentFolder = normalizeSentFolder(req.SentFolder)
	req.SignatureHTML = strings.TrimSpace(req.SignatureHTML)
	if req.DisplayName == "" || req.Email == "" || req.IMAPHost == "" || req.IMAPUsername == "" || req.IMAPPort <= 0 {
		response.Error(w, http.StatusBadRequest, "邮箱账号配置不完整")
		return
	}
	if req.SMTPHost != "" && req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}
	if req.PollIntervalMinutes <= 0 {
		req.PollIntervalMinutes = 10
	}
	if req.CleanupRetentionDays <= 0 {
		req.CleanupRetentionDays = 90
	}
	if len([]rune(req.SignatureHTML)) > 10000 {
		response.Error(w, http.StatusBadRequest, "签名模板不能超过 10000 个字符")
		return
	}

	encryptedPassword := current.IMAPPasswordEncoded
	if strings.TrimSpace(req.IMAPPassword) != "" {
		encryptedPassword, err = crypto.EncryptString(req.IMAPPassword, a.cfg.App.CredentialSecret)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "邮箱凭据加密失败")
			return
		}
	}
	encryptedSMTPPassword := current.SMTPPasswordEncoded
	if strings.TrimSpace(req.SMTPPassword) != "" {
		encryptedSMTPPassword, err = crypto.EncryptString(req.SMTPPassword, a.cfg.App.CredentialSecret)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "SMTP 凭据加密失败")
			return
		}
	} else if req.SMTPUseIMAPPassword && req.SMTPHost != "" {
		encryptedSMTPPassword = encryptedPassword
	}

	current.DisplayName = req.DisplayName
	current.Email = req.Email
	current.IMAPHost = req.IMAPHost
	current.IMAPPort = req.IMAPPort
	current.IMAPTLS = req.IMAPTLS
	current.IMAPUsername = req.IMAPUsername
	current.IMAPPasswordEncoded = encryptedPassword
	current.SMTPHost = req.SMTPHost
	current.SMTPPort = req.SMTPPort
	current.SMTPTLS = req.SMTPTLS
	current.SMTPStartTLS = req.SMTPStartTLS
	current.SMTPUsername = req.SMTPUsername
	current.SMTPPasswordEncoded = encryptedSMTPPassword
	current.SentFolder = req.SentFolder
	current.SignatureHTML = req.SignatureHTML
	current.PollIntervalMinutes = req.PollIntervalMinutes
	current.Enabled = req.Enabled
	current.CleanupEnabled = req.CleanupEnabled
	current.CleanupRetentionDays = req.CleanupRetentionDays

	account, err := a.store.UpdateMailAccount(current)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "更新邮箱账号失败")
		return
	}

	response.OK(w, "更新成功", mailAccountPayload(account))
}

func (a *App) handleDeleteMailAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮箱账号 ID 格式错误")
		return
	}

	if err := a.store.DeleteMailAccount(userID, id); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "删除邮箱账号失败")
		return
	}

	response.OK(w, "删除成功", nil)
}

func (a *App) handleTestMailAccountConnection(w http.ResponseWriter, r *http.Request) {
	userID, accountID, ok := accountRouteIDs(w, r)
	if !ok {
		return
	}

	if err := a.mailService.TestConnection(userID, accountID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusBadRequest, "邮箱连接失败："+err.Error())
		return
	}

	response.OK(w, "连接成功", nil)
}

func (a *App) handleListMailAccountFolders(w http.ResponseWriter, r *http.Request) {
	userID, accountID, ok := accountRouteIDs(w, r)
	if !ok {
		return
	}

	folders, err := a.mailService.ListFolders(userID, accountID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "读取邮箱文件夹失败："+err.Error())
		return
	}

	items := make([]map[string]any, 0, len(folders))
	for _, folder := range folders {
		items = append(items, map[string]any{
			"name":          folder.Name,
			"delimiter":     folder.Delimiter,
			"attributes":    folder.Attributes,
			"sentCandidate": mail.IsSentFolderCandidate(folder),
		})
	}
	response.OK(w, "获取成功", map[string]any{"items": items})
}

func (a *App) handleSyncMailAccount(w http.ResponseWriter, r *http.Request) {
	userID, accountID, ok := accountRouteIDs(w, r)
	if !ok {
		return
	}

	result, err := a.mailService.SyncInbox(userID, accountID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if errors.Is(err, mail.ErrSyncAlreadyRunning) {
		response.Error(w, http.StatusConflict, "邮箱账号正在收取中，请稍后再试")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "收取失败："+err.Error())
		return
	}

	response.OK(w, "收取完成", map[string]any{
		"jobId":           strconv.FormatInt(result.JobID, 10),
		"newMessageCount": result.NewMessageCount,
		"warnings":        result.Warnings,
	})
}

func (a *App) handleStartFullSyncMailAccount(w http.ResponseWriter, r *http.Request) {
	userID, accountID, ok := accountRouteIDs(w, r)
	if !ok {
		return
	}

	status, err := a.mailService.StartFullSync(userID, accountID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "启动全量同步失败："+err.Error())
		return
	}

	response.JSON(w, http.StatusAccepted, true, "已开始全量同步", fullSyncStatusPayload(status))
}

func (a *App) handleStopFullSyncMailAccount(w http.ResponseWriter, r *http.Request) {
	userID, accountID, ok := accountRouteIDs(w, r)
	if !ok {
		return
	}

	status, err := a.mailService.StopFullSync(userID, accountID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "停止全量同步失败："+err.Error())
		return
	}

	response.OK(w, "已停止全量同步", fullSyncStatusPayload(status))
}

func (a *App) handleMailAccountSyncStatus(w http.ResponseWriter, r *http.Request) {
	userID, accountID, ok := accountRouteIDs(w, r)
	if !ok {
		return
	}

	status, err := a.mailService.GetFullSyncStatus(userID, accountID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取同步状态失败")
		return
	}

	response.OK(w, "获取成功", fullSyncStatusPayload(status))
}

func (a *App) handleListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	started := time.Now()

	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	folderID, _ := strconv.ParseInt(r.URL.Query().Get("folderId"), 10, 64)
	offset := (page - 1) * pageSize

	messages, total, err := a.store.ListMailMessagesByQuery(storage.ListMailMessagesQuery{
		UserID:         userID,
		AccountID:      accountID,
		FolderID:       folderID,
		SystemFolder:   r.URL.Query().Get("systemFolder"),
		Keyword:        r.URL.Query().Get("keyword"),
		From:           r.URL.Query().Get("from"),
		Subject:        r.URL.Query().Get("subject"),
		Body:           r.URL.Query().Get("body"),
		DateFrom:       parseDateQuery(r.URL.Query().Get("dateFrom")),
		DateTo:         parseDateQuery(r.URL.Query().Get("dateTo")),
		HasAttachments: parseBoolQuery(r.URL.Query().Get("hasAttachments")),
		Limit:          pageSize,
		Offset:         offset,
		SummaryOnly:    true,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件列表失败")
		return
	}
	queryDuration := time.Since(started)

	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		items = append(items, messageListPayload(message))
	}

	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
	logSlowAPI("messages.list", started, "query="+queryDuration.String(), "items="+strconv.Itoa(len(items)), "total="+strconv.Itoa(total))
}

func (a *App) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	req, attachments, err := decodeSendMessageRequest(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	accountID, err := strconv.ParseInt(strings.TrimSpace(req.AccountID), 10, 64)
	if err != nil || accountID <= 0 {
		response.Error(w, http.StatusBadRequest, "请选择发件邮箱账号")
		return
	}
	to, ok := normalizeOutgoingAddresses(w, req.To, "收件人")
	if !ok {
		return
	}
	cc, ok := normalizeOutgoingAddresses(w, req.CC, "抄送人")
	if !ok {
		return
	}
	bcc, ok := normalizeOutgoingAddresses(w, req.BCC, "密送人")
	if !ok {
		return
	}
	if len(to) == 0 && len(cc) == 0 && len(bcc) == 0 {
		response.Error(w, http.StatusBadRequest, "至少需要填写一个收件人")
		return
	}
	req.Subject = strings.TrimSpace(req.Subject)
	req.TextBody = strings.TrimSpace(req.TextBody)
	req.HTMLBody = strings.TrimSpace(req.HTMLBody)
	if req.Subject == "" && req.TextBody == "" && req.HTMLBody == "" {
		response.Error(w, http.StatusBadRequest, "主题和正文不能同时为空")
		return
	}
	if len([]rune(req.Subject)) > 500 {
		response.Error(w, http.StatusBadRequest, "邮件主题不能超过 500 个字符")
		return
	}

	sent, err := a.mailService.SendMessage(userID, accountID, mail.OutgoingMessage{
		To:          to,
		CC:          cc,
		BCC:         bcc,
		Subject:     req.Subject,
		TextBody:    req.TextBody,
		HTMLBody:    req.HTMLBody,
		Attachments: attachments,
	})
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "发送失败："+err.Error())
		return
	}
	response.OK(w, "发送成功", messageListPayload(sent))
}

func (a *App) handleMessageDetail(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	started := time.Now()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}

	segmentStarted := time.Now()
	message, err := a.store.FindMailMessageByID(userID, id)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件详情失败")
		return
	}
	messageDuration := time.Since(segmentStarted)

	payload := messageListPayload(message)
	segmentStarted = time.Now()
	attachments, err := a.store.ListMailAttachments(userID, message.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件附件失败")
		return
	}
	attachmentsDuration := time.Since(segmentStarted)
	segmentStarted = time.Now()
	htmlBody := readOptionalFile(message.HTMLBodyPath)
	htmlReadDuration := time.Since(segmentStarted)
	segmentStarted = time.Now()
	inlineContentIDs := referencedInlineContentIDs(htmlBody, attachments)
	payload["textBody"] = readOptionalFile(message.TextBodyPath)
	bodyReadDuration := time.Since(segmentStarted)
	segmentStarted = time.Now()
	payload["htmlBody"] = rewriteInlineCIDImages(htmlBody, attachments, inlineContentIDs, userID, message.ID, a.cfg.App.JWTSecret)
	rewriteDuration := time.Since(segmentStarted)
	payload["cc"] = splitAddressField(message.CCAddrs)
	payload["folder"] = message.Folder
	payload["messageId"] = nullableString(message.MessageID)
	payload["attachments"] = attachmentPayloads(message.ID, attachments, inlineContentIDs)

	response.OK(w, "获取成功", payload)
	logSlowAPI(
		"messages.detail",
		started,
		"id="+strconv.FormatInt(id, 10),
		"message="+messageDuration.String(),
		"attachments="+attachmentsDuration.String(),
		"htmlRead="+htmlReadDuration.String(),
		"bodyRead="+bodyReadDuration.String(),
		"cidRewrite="+rewriteDuration.String(),
		"attachmentCount="+strconv.Itoa(len(attachments)),
		"htmlBytes="+strconv.Itoa(len(htmlBody)),
	)
}

func (a *App) handleAssignMessageFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	messageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}
	var req assignMessageFolderRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	var folderID sql.NullInt64
	if strings.TrimSpace(req.FolderID) != "" {
		parsedFolderID, err := strconv.ParseInt(req.FolderID, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "文件夹 ID 格式错误")
			return
		}
		if _, err := a.store.FindMailFolderByID(userID, parsedFolderID); errors.Is(err, storage.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "文件夹不存在")
			return
		} else if err != nil {
			response.Error(w, http.StatusInternalServerError, "获取文件夹失败")
			return
		}
		folderID = sql.NullInt64{Int64: parsedFolderID, Valid: true}
	}

	if err := a.store.UpdateMailMessageFolder(userID, messageID, folderID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "更新邮件文件夹失败")
		return
	}
	response.OK(w, "更新成功", nil)
}

func (a *App) handleListContacts(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 100)
	contacts, total, err := a.store.ListContacts(storage.ListContactsQuery{
		UserID:  userID,
		Keyword: r.URL.Query().Get("keyword"),
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取联系人失败")
		return
	}
	items := make([]map[string]any, 0, len(contacts))
	for _, contact := range contacts {
		items = append(items, contactPayload(contact))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}

func (a *App) handleCreateContact(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req contactRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	params, ok := contactParamsFromRequest(w, userID, req)
	if !ok {
		return
	}
	contact, err := a.store.CreateContact(params)
	if err != nil {
		response.Error(w, http.StatusConflict, "联系人邮箱已存在")
		return
	}
	response.Created(w, "创建成功", contactPayload(contact))
}

func (a *App) handleUpdateContact(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "联系人 ID 格式错误")
		return
	}
	current, err := a.store.FindContactByID(userID, id)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "联系人不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取联系人失败")
		return
	}
	var req contactRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	params, ok := contactParamsFromRequest(w, userID, req)
	if !ok {
		return
	}
	current.Email = params.Email
	current.EmailKey = strings.ToLower(params.Email)
	current.DisplayName = sql.NullString{String: params.DisplayName, Valid: strings.TrimSpace(params.DisplayName) != ""}
	current.Nickname = sql.NullString{String: params.Nickname, Valid: strings.TrimSpace(params.Nickname) != ""}
	current.Phone = sql.NullString{String: params.Phone, Valid: strings.TrimSpace(params.Phone) != ""}
	current.Company = sql.NullString{String: params.Company, Valid: strings.TrimSpace(params.Company) != ""}
	current.Notes = sql.NullString{String: params.Notes, Valid: strings.TrimSpace(params.Notes) != ""}
	current.Source = "manual"
	contact, err := a.store.UpdateContact(current)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "联系人不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusConflict, "联系人邮箱已存在")
		return
	}
	response.OK(w, "更新成功", contactPayload(contact))
}

func (a *App) handleDeleteContact(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "联系人 ID 格式错误")
		return
	}
	if err := a.store.DeleteContact(userID, id); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "联系人不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "删除联系人失败")
		return
	}
	response.OK(w, "删除成功", nil)
}

func (a *App) handleListMailFolders(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	folders, err := a.store.ListMailFolders(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取文件夹失败")
		return
	}
	items := make([]map[string]any, 0, len(folders))
	for _, folder := range folders {
		items = append(items, mailFolderPayload(folder))
	}
	response.OK(w, "获取成功", map[string]any{"items": items})
}

func (a *App) handleCreateMailFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req createMailFolderRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Color = strings.TrimSpace(req.Color)
	if req.Name == "" {
		response.Error(w, http.StatusBadRequest, "文件夹名称不能为空")
		return
	}
	folder, err := a.store.CreateMailFolder(storage.CreateMailFolderParams{
		UserID:    userID,
		Name:      req.Name,
		Color:     req.Color,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		response.Error(w, http.StatusConflict, "文件夹名称已存在或创建失败")
		return
	}
	response.Created(w, "创建成功", mailFolderPayload(folder))
}

func (a *App) handleDeleteMailFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	folderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "文件夹 ID 格式错误")
		return
	}
	if err := a.store.DeleteMailFolder(userID, folderID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "文件夹不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "删除文件夹失败")
		return
	}
	response.OK(w, "删除成功", nil)
}

func (a *App) handleListMailRules(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	rules, err := a.store.ListMailRules(userID, false)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取规则失败")
		return
	}
	items := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		items = append(items, mailRulePayload(rule))
	}
	response.OK(w, "获取成功", map[string]any{"items": items})
}

func (a *App) handleCreateMailRule(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req createMailRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	params, ok := a.mailRuleParamsFromRequest(w, userID, req)
	if !ok {
		return
	}
	rule, err := a.store.CreateMailRule(params)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "创建规则失败")
		return
	}
	response.Created(w, "创建成功", mailRulePayload(rule))
}

func (a *App) handleUpdateMailRule(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	ruleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "规则 ID 格式错误")
		return
	}
	var req createMailRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	params, ok := a.mailRuleParamsFromRequest(w, userID, req)
	if !ok {
		return
	}
	rule, err := a.store.UpdateMailRule(userID, ruleID, params)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "规则不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "更新规则失败")
		return
	}
	response.OK(w, "更新成功", mailRulePayload(rule))
}

func (a *App) mailRuleParamsFromRequest(w http.ResponseWriter, userID int64, req createMailRuleRequest) (storage.CreateMailRuleParams, bool) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		response.Error(w, http.StatusBadRequest, "规则名称不能为空")
		return storage.CreateMailRuleParams{}, false
	}
	targetFolderID, err := strconv.ParseInt(req.TargetFolderID, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "目标文件夹 ID 格式错误")
		return storage.CreateMailRuleParams{}, false
	}
	if _, err := a.store.FindMailFolderByID(userID, targetFolderID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "目标文件夹不存在")
		return storage.CreateMailRuleParams{}, false
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取目标文件夹失败")
		return storage.CreateMailRuleParams{}, false
	}
	matchMode := strings.ToLower(strings.TrimSpace(req.MatchMode))
	if matchMode != "any" {
		matchMode = "all"
	}
	conditions := make([]storage.CreateMailRuleConditionParams, 0, len(req.Conditions))
	for _, condition := range req.Conditions {
		field := strings.TrimSpace(condition.Field)
		operator := strings.TrimSpace(condition.Operator)
		if field == "" || operator == "" {
			response.Error(w, http.StatusBadRequest, "规则条件不完整")
			return storage.CreateMailRuleParams{}, false
		}
		conditions = append(conditions, storage.CreateMailRuleConditionParams{
			Field:    field,
			Operator: operator,
			Value:    strings.TrimSpace(condition.Value),
		})
	}
	return storage.CreateMailRuleParams{
		UserID:         userID,
		Name:           req.Name,
		Enabled:        req.Enabled,
		MatchMode:      matchMode,
		TargetFolderID: targetFolderID,
		SortOrder:      req.SortOrder,
		Conditions:     conditions,
	}, true
}

func (a *App) handleDeleteMailRule(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	ruleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "规则 ID 格式错误")
		return
	}
	if err := a.store.DeleteMailRule(userID, ruleID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "规则不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "删除规则失败")
		return
	}
	response.OK(w, "删除成功", nil)
}

func (a *App) handleApplyMailRules(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req applyMailRulesRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	scope := mail.RuleApplyScope(strings.ToLower(strings.TrimSpace(req.Scope)))
	if scope != mail.RuleApplyScopeAll {
		scope = mail.RuleApplyScopeUnfiled
	}
	count, err := a.mailService.ApplyRules(userID, scope)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "应用规则失败")
		return
	}
	response.OK(w, "应用完成", map[string]any{"appliedCount": count})
}

func (a *App) handleAttachmentContent(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	messageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}
	attachmentID, err := strconv.ParseInt(r.PathValue("attachmentId"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "附件 ID 格式错误")
		return
	}

	if _, err := a.store.FindMailMessageByID(userID, messageID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件失败")
		return
	}

	attachment, err := a.store.FindMailAttachmentByID(userID, messageID, attachmentID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "附件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取附件失败")
		return
	}

	contentType := "application/octet-stream"
	if attachment.ContentType.Valid && strings.TrimSpace(attachment.ContentType.String) != "" {
		contentType = attachment.ContentType.String
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Filename}))
	http.ServeFile(w, r, attachment.FilePath)
}

func (a *App) handleInlineAttachmentContent(w http.ResponseWriter, r *http.Request) {
	messageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}
	attachmentID, err := strconv.ParseInt(r.PathValue("attachmentId"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "附件 ID 格式错误")
		return
	}
	userID, err := strconv.ParseInt(r.URL.Query().Get("uid"), 10, 64)
	if err != nil || userID <= 0 {
		response.Error(w, http.StatusUnauthorized, "内嵌图片链接已失效")
		return
	}
	expiresAt, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		response.Error(w, http.StatusUnauthorized, "内嵌图片链接已失效")
		return
	}
	signature := r.URL.Query().Get("sig")
	if !validInlineAttachmentSignature(a.cfg.App.JWTSecret, userID, messageID, attachmentID, expiresAt, signature) {
		response.Error(w, http.StatusUnauthorized, "内嵌图片链接已失效")
		return
	}

	attachment, err := a.store.FindMailAttachmentByID(userID, messageID, attachmentID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "附件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取附件失败")
		return
	}

	contentType := "application/octet-stream"
	if attachment.ContentType.Valid && strings.TrimSpace(attachment.ContentType.String) != "" {
		contentType = attachment.ContentType.String
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": attachment.Filename}))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, attachment.FilePath)
}

func (a *App) handleMicrosoftOAuthStart(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	if strings.TrimSpace(a.cfg.OAuth.Microsoft.ClientID) == "" {
		response.Error(w, http.StatusBadRequest, "Microsoft OAuth 未配置 clientId，请先配置 config.yaml")
		return
	}

	start, err := a.oauth.StartMicrosoft(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "创建 Microsoft 授权链接失败")
		return
	}

	response.OK(w, "获取成功", map[string]any{
		"state":   start.State,
		"authUrl": start.AuthURL,
	})
}

func (a *App) handleMicrosoftOAuthComplete(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	var req completeMicrosoftOAuthRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	account, err := a.oauth.CompleteMicrosoft(userID, req.Code, req.State)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Microsoft 授权失败："+err.Error())
		return
	}

	response.OK(w, "授权成功", mailAccountPayload(account))
}

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		tokenValue, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || strings.TrimSpace(tokenValue) == "" {
			response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}

		userID, err := auth.ParseToken(tokenValue, a.cfg.App.JWTSecret)
		if err != nil {
			response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) issueToken(userID int64) (string, error) {
	expireHours := a.cfg.App.JWTExpireHours
	if expireHours <= 0 {
		expireHours = 168
	}
	return auth.GenerateToken(userID, a.cfg.App.JWTSecret, time.Duration(expireHours)*time.Hour)
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func decodeSendMessageRequest(r *http.Request) (sendMessageRequest, []mail.OutgoingAttachment, error) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		var req sendMessageRequest
		if err := decodeJSON(r, &req); err != nil {
			return sendMessageRequest{}, nil, errors.New("请求参数格式错误")
		}
		return req, nil, nil
	}

	if err := r.ParseMultipartForm(maxComposeAttachmentBytes + 1<<20); err != nil {
		return sendMessageRequest{}, nil, errors.New("读取发信表单失败")
	}
	form := r.MultipartForm
	req := sendMessageRequest{
		AccountID: strings.TrimSpace(formValue(form, "accountId")),
		To:        formAddressValues(form, "to"),
		CC:        formAddressValues(form, "cc"),
		BCC:       formAddressValues(form, "bcc"),
		Subject:   formValue(form, "subject"),
		TextBody:  formValue(form, "textBody"),
		HTMLBody:  formValue(form, "htmlBody"),
	}
	attachments, err := readComposeAttachments(form)
	if err != nil {
		return sendMessageRequest{}, nil, err
	}
	return req, attachments, nil
}

func formValue(form *multipart.Form, key string) string {
	if form == nil || len(form.Value[key]) == 0 {
		return ""
	}
	return form.Value[key][0]
}

func formAddressValues(form *multipart.Form, key string) []string {
	raw := strings.TrimSpace(formValue(form, key))
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '，' || r == '；'
	})
}

func readComposeAttachments(form *multipart.Form) ([]mail.OutgoingAttachment, error) {
	if form == nil {
		return nil, nil
	}
	fileHeaders := form.File["attachments"]
	if len(fileHeaders) > maxComposeAttachmentCount {
		return nil, fmt.Errorf("附件不能超过 %d 个", maxComposeAttachmentCount)
	}
	attachments := make([]mail.OutgoingAttachment, 0, len(fileHeaders))
	var total int64
	for _, header := range fileHeaders {
		if header == nil || strings.TrimSpace(header.Filename) == "" {
			continue
		}
		file, err := header.Open()
		if err != nil {
			return nil, errors.New("读取附件失败")
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxComposeAttachmentBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, errors.New("读取附件失败")
		}
		total += int64(len(data))
		if total > maxComposeAttachmentBytes {
			return nil, fmt.Errorf("附件总大小不能超过 %d MB", maxComposeAttachmentBytes>>20)
		}
		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		attachments = append(attachments, mail.OutgoingAttachment{
			Filename:    filepath.Base(header.Filename),
			ContentType: contentType,
			Data:        data,
		})
	}
	return attachments, nil
}

func userPayload(user storage.User) map[string]any {
	return map[string]any{
		"id":        strconv.FormatInt(user.ID, 10),
		"username":  user.Username,
		"email":     user.Email,
		"nickname":  nullableString(user.Nickname),
		"bio":       nullableString(user.Bio),
		"avatarUrl": profileAvatarURL(user),
	}
}

func profileAvatarURL(user storage.User) any {
	if !user.AvatarPath.Valid || strings.TrimSpace(user.AvatarPath.String) == "" {
		return nil
	}
	return "/api/v1/profile/avatar/content"
}

func currentUserID(r *http.Request) (int64, bool) {
	userID, ok := r.Context().Value(userIDKey).(int64)
	return userID, ok
}

func accountRouteIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return 0, 0, false
	}
	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮箱账号 ID 格式错误")
		return 0, 0, false
	}
	return userID, accountID, true
}

func mailAccountPayload(account storage.MailAccount) map[string]any {
	var lastSyncAt any
	if account.LastSyncAt.Valid {
		lastSyncAt = account.LastSyncAt.Time
	}

	var lastSyncStatus any
	if account.LastSyncStatus.Valid {
		lastSyncStatus = account.LastSyncStatus.String
	}

	var lastSyncError any
	if account.LastSyncError.Valid {
		lastSyncError = account.LastSyncError.String
	}

	return map[string]any{
		"id":                   strconv.FormatInt(account.ID, 10),
		"provider":             account.Provider,
		"authType":             account.AuthType,
		"displayName":          account.DisplayName,
		"email":                account.Email,
		"imapHost":             account.IMAPHost,
		"imapPort":             account.IMAPPort,
		"imapTls":              account.IMAPTLS,
		"imapUsername":         account.IMAPUsername,
		"smtpHost":             account.SMTPHost,
		"smtpPort":             account.SMTPPort,
		"smtpTls":              account.SMTPTLS,
		"smtpStartTls":         account.SMTPStartTLS,
		"smtpUsername":         account.SMTPUsername,
		"smtpConfigured":       strings.TrimSpace(account.SMTPHost) != "",
		"sentFolder":           normalizeSentFolder(account.SentFolder),
		"signatureHtml":        account.SignatureHTML,
		"pollIntervalMinutes":  account.PollIntervalMinutes,
		"enabled":              account.Enabled,
		"lastSyncAt":           lastSyncAt,
		"lastSyncStatus":       lastSyncStatus,
		"lastSyncError":        lastSyncError,
		"fullSyncStatus":       account.FullSyncStatus,
		"fullSyncTotal":        account.FullSyncTotal,
		"fullSyncProcessed":    account.FullSyncProcessed,
		"fullSyncNewCount":     account.FullSyncNewCount,
		"fullSyncStartedAt":    nullableTime(account.FullSyncStartedAt),
		"fullSyncFinishedAt":   nullableTime(account.FullSyncFinishedAt),
		"fullSyncError":        nullableString(account.FullSyncError),
		"cleanupEnabled":       account.CleanupEnabled,
		"cleanupRetentionDays": account.CleanupRetentionDays,
	}
}

func fullSyncStatusPayload(status mail.FullSyncStatus) map[string]any {
	return map[string]any{
		"fullSyncStatus":       status.Status,
		"fullSyncTotal":        status.Total,
		"fullSyncProcessed":    status.Processed,
		"fullSyncNewCount":     status.NewCount,
		"fullSyncStartedAt":    nullableTime(status.StartedAt),
		"fullSyncFinishedAt":   nullableTime(status.FinishedAt),
		"fullSyncError":        nullableString(status.Error),
		"cleanupEnabled":       status.CleanupEnabled,
		"cleanupRetentionDays": status.RetentionDays,
	}
}

func messageListPayload(message storage.MailMessage) map[string]any {
	return map[string]any{
		"id":             strconv.FormatInt(message.ID, 10),
		"accountId":      strconv.FormatInt(message.AccountID, 10),
		"localFolderId":  nullableInt64(message.LocalFolderID),
		"subject":        nullableString(message.Subject),
		"from":           nullableString(message.FromAddr),
		"to":             splitAddressField(message.ToAddrs),
		"sentAt":         nullableTime(message.SentAt),
		"receivedAt":     nullableTime(message.ReceivedAt),
		"hasAttachments": message.HasAttachments,
	}
}

func mailFolderPayload(folder storage.MailFolder) map[string]any {
	return map[string]any{
		"id":        strconv.FormatInt(folder.ID, 10),
		"name":      folder.Name,
		"color":     nullableString(folder.Color),
		"sortOrder": folder.SortOrder,
	}
}

func contactPayload(contact storage.Contact) map[string]any {
	displayName := nullableString(contact.DisplayName)
	nickname := nullableString(contact.Nickname)
	preferredName := contact.Email
	if name, ok := nickname.(string); ok && strings.TrimSpace(name) != "" {
		preferredName = name
	} else if name, ok := displayName.(string); ok && strings.TrimSpace(name) != "" {
		preferredName = name
	}
	return map[string]any{
		"id":          strconv.FormatInt(contact.ID, 10),
		"email":       contact.Email,
		"displayName": displayName,
		"nickname":    nickname,
		"name":        preferredName,
		"phone":       nullableString(contact.Phone),
		"company":     nullableString(contact.Company),
		"notes":       nullableString(contact.Notes),
		"source":      contact.Source,
		"firstSeenAt": nullableTime(contact.FirstSeenAt),
		"lastSeenAt":  nullableTime(contact.LastSeenAt),
		"createdAt":   contact.CreatedAt,
		"updatedAt":   contact.UpdatedAt,
	}
}

func contactParamsFromRequest(w http.ResponseWriter, userID int64, req contactRequest) (storage.CreateContactParams, bool) {
	email, displayFromEmail, ok := normalizeContactEmail(w, req.Email)
	if !ok {
		return storage.CreateContactParams{}, false
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		req.DisplayName = displayFromEmail
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Company = strings.TrimSpace(req.Company)
	req.Notes = strings.TrimSpace(req.Notes)
	if len([]rune(req.DisplayName)) > 80 {
		response.Error(w, http.StatusBadRequest, "联系人姓名不能超过 80 个字符")
		return storage.CreateContactParams{}, false
	}
	if len([]rune(req.Nickname)) > 80 {
		response.Error(w, http.StatusBadRequest, "联系人昵称不能超过 80 个字符")
		return storage.CreateContactParams{}, false
	}
	if len([]rune(req.Phone)) > 40 {
		response.Error(w, http.StatusBadRequest, "联系电话不能超过 40 个字符")
		return storage.CreateContactParams{}, false
	}
	if len([]rune(req.Company)) > 120 {
		response.Error(w, http.StatusBadRequest, "公司不能超过 120 个字符")
		return storage.CreateContactParams{}, false
	}
	if len([]rune(req.Notes)) > 500 {
		response.Error(w, http.StatusBadRequest, "备注不能超过 500 个字符")
		return storage.CreateContactParams{}, false
	}
	return storage.CreateContactParams{
		UserID:      userID,
		Email:       email,
		DisplayName: req.DisplayName,
		Nickname:    req.Nickname,
		Phone:       req.Phone,
		Company:     req.Company,
		Notes:       req.Notes,
		Source:      "manual",
		SeenAt:      sql.NullTime{Time: time.Now(), Valid: true},
	}, true
}

func normalizeContactEmail(w http.ResponseWriter, value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		response.Error(w, http.StatusBadRequest, "联系人邮箱不能为空")
		return "", "", false
	}
	address, err := netmail.ParseAddress(value)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		response.Error(w, http.StatusBadRequest, "联系人邮箱格式不正确")
		return "", "", false
	}
	email := strings.ToLower(strings.TrimSpace(address.Address))
	if len([]rune(email)) > 254 {
		response.Error(w, http.StatusBadRequest, "联系人邮箱过长")
		return "", "", false
	}
	return email, strings.TrimSpace(address.Name), true
}

func normalizeOutgoingAddresses(w http.ResponseWriter, values []string, label string) ([]string, bool) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		addresses, err := netmail.ParseAddressList(value)
		if err != nil {
			response.Error(w, http.StatusBadRequest, label+"邮箱格式不正确")
			return nil, false
		}
		for _, address := range addresses {
			if strings.TrimSpace(address.Address) == "" {
				response.Error(w, http.StatusBadRequest, label+"邮箱格式不正确")
				return nil, false
			}
			out = append(out, address.String())
		}
	}
	if len(out) > 200 {
		response.Error(w, http.StatusBadRequest, label+"不能超过 200 个")
		return nil, false
	}
	return out, true
}

func mailRulePayload(rule storage.MailRule) map[string]any {
	conditions := make([]map[string]any, 0, len(rule.Conditions))
	for _, condition := range rule.Conditions {
		conditions = append(conditions, map[string]any{
			"id":       strconv.FormatInt(condition.ID, 10),
			"field":    condition.Field,
			"operator": condition.Operator,
			"value":    condition.Value,
		})
	}
	return map[string]any{
		"id":             strconv.FormatInt(rule.ID, 10),
		"name":           rule.Name,
		"enabled":        rule.Enabled,
		"matchMode":      rule.MatchMode,
		"targetFolderId": strconv.FormatInt(rule.TargetFolderID, 10),
		"sortOrder":      rule.SortOrder,
		"conditions":     conditions,
	}
}

func attachmentPayloads(messageID int64, attachments []storage.MailAttachment, inlineContentIDs map[string]bool) []map[string]any {
	items := make([]map[string]any, 0, len(attachments))
	for _, attachment := range attachments {
		inline := attachment.Inline || inlineContentIDs[normalizeContentID(nullableStringValue(attachment.ContentID))]
		items = append(items, map[string]any{
			"id":          strconv.FormatInt(attachment.ID, 10),
			"messageId":   strconv.FormatInt(messageID, 10),
			"filename":    attachment.Filename,
			"contentType": nullableString(attachment.ContentType),
			"contentId":   nullableString(attachment.ContentID),
			"inline":      inline,
			"size":        attachment.Size,
			"downloadUrl": fmt.Sprintf("/api/v1/messages/%d/attachments/%d/content", messageID, attachment.ID),
		})
	}
	return items
}

func rewriteInlineCIDImages(htmlBody string, attachments []storage.MailAttachment, inlineContentIDs map[string]bool, userID, messageID int64, secret string) string {
	if strings.TrimSpace(htmlBody) == "" {
		return htmlBody
	}
	replacements := make(map[string]string)
	for _, attachment := range attachments {
		contentID := nullableStringValue(attachment.ContentID)
		normalizedContentID := normalizeContentID(contentID)
		if normalizedContentID == "" || (!attachment.Inline && !inlineContentIDs[normalizedContentID]) {
			continue
		}
		contentType := "application/octet-stream"
		if attachment.ContentType.Valid && strings.TrimSpace(attachment.ContentType.String) != "" {
			contentType = attachment.ContentType.String
		}
		if browserCanDisplayImage(contentType, attachment.FilePath) {
			replacements[normalizedContentID] = inlineAttachmentContentURL(secret, userID, messageID, attachment.ID)
			continue
		}
		data, err := os.ReadFile(attachment.FilePath)
		if err != nil {
			continue
		}
		contentType, data = browserDisplayableInlineImage(contentType, attachment.FilePath, data)
		replacements[normalizedContentID] = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return rewriteCIDReferences(htmlBody, replacements)
}

func referencedInlineContentIDs(htmlBody string, attachments []storage.MailAttachment) map[string]bool {
	referenced := make(map[string]bool)
	if strings.TrimSpace(htmlBody) == "" || len(attachments) == 0 {
		return referenced
	}
	htmlContentIDs := extractCIDReferences(htmlBody)
	for _, attachment := range attachments {
		contentID := nullableStringValue(attachment.ContentID)
		normalizedContentID := normalizeContentID(contentID)
		if normalizedContentID == "" {
			continue
		}
		if htmlContentIDs[normalizedContentID] {
			referenced[normalizedContentID] = true
		}
	}
	return referenced
}

func rewriteCIDReferences(htmlBody string, replacements map[string]string) string {
	return cidReferencePattern.ReplaceAllStringFunc(htmlBody, func(reference string) string {
		normalizedContentID := normalizeContentID(reference)
		if replacement, ok := replacements[normalizedContentID]; ok {
			return replacement
		}
		return missingInlineImagePlaceholderDataURL()
	})
}

func inlineAttachmentContentURL(secret string, userID, messageID, attachmentID int64) string {
	expiresAt := time.Now().Add(inlineAttachmentURLTTL).Unix()
	signature := inlineAttachmentSignature(secret, userID, messageID, attachmentID, expiresAt)
	return fmt.Sprintf(
		"/api/v1/messages/%d/attachments/%d/inline-content?uid=%d&exp=%d&sig=%s",
		messageID,
		attachmentID,
		userID,
		expiresAt,
		url.QueryEscape(signature),
	)
}

func validInlineAttachmentSignature(secret string, userID, messageID, attachmentID, expiresAt int64, signature string) bool {
	if strings.TrimSpace(signature) == "" {
		return false
	}
	expected := inlineAttachmentSignature(secret, userID, messageID, attachmentID, expiresAt)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func inlineAttachmentSignature(secret string, userID, messageID, attachmentID, expiresAt int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d:%d:%d:%d", userID, messageID, attachmentID, expiresAt)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func extractCIDReferences(htmlBody string) map[string]bool {
	contentIDs := make(map[string]bool)
	for _, reference := range cidReferencePattern.FindAllString(htmlBody, -1) {
		normalizedContentID := normalizeContentID(reference)
		if normalizedContentID != "" {
			contentIDs[normalizedContentID] = true
		}
	}
	return contentIDs
}

func normalizeContentID(contentID string) string {
	contentID = strings.TrimSpace(contentID)
	if decoded, err := url.PathUnescape(contentID); err == nil {
		contentID = decoded
	}
	contentID = strings.TrimSpace(contentID)
	if strings.HasPrefix(strings.ToLower(contentID), "cid:") {
		contentID = strings.TrimSpace(contentID[4:])
	}
	contentID = strings.TrimPrefix(strings.TrimSuffix(contentID, ">"), "<")
	return strings.ToLower(contentID)
}

func normalizeSentFolder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Sent"
	}
	return value
}

func browserDisplayableInlineImage(contentType string, filePath string, data []byte) (string, []byte) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	extension := strings.ToLower(filepath.Ext(filePath))
	if mediaType != "image/tiff" && mediaType != "image/tif" && extension != ".tif" && extension != ".tiff" {
		return contentType, data
	}

	image, err := tiff.Decode(bytes.NewReader(data))
	if err != nil {
		return contentType, data
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image); err != nil {
		return contentType, data
	}
	return "image/png", pngData.Bytes()
}

func browserCanDisplayImage(contentType string, filePath string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch mediaType {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp", "image/svg+xml", "image/bmp", "image/x-icon":
		return true
	case "image/tiff", "image/tif":
		return false
	}
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return true
	default:
		return false
	}
}

func missingInlineImagePlaceholderDataURL() string {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="220" height="48" viewBox="0 0 220 48"><rect width="220" height="48" rx="6" fill="#f8fafc" stroke="#cbd5e1"/><text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" fill="#64748b" font-size="14" font-family="Arial, sans-serif">内嵌图片缺失</text></svg>`
	return "data:image/svg+xml;charset=utf-8," + url.PathEscape(svg)
}

func nullableStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullableString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func nullableTime(value sql.NullTime) any {
	if value.Valid {
		return value.Time
	}
	return nil
}

func nullableInt64(value sql.NullInt64) any {
	if value.Valid {
		return strconv.FormatInt(value.Int64, 10)
	}
	return nil
}

func splitAddressField(value sql.NullString) []string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return []string{}
	}
	parts := strings.Split(value.String, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func readOptionalFile(value sql.NullString) string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return ""
	}
	content, err := os.ReadFile(value.String)
	if err != nil {
		return ""
	}
	return string(content)
}

func logSlowAPI(name string, started time.Time, details ...string) {
	duration := time.Since(started)
	if duration < 500*time.Millisecond {
		return
	}
	fields := []string{"slow api", "name=" + name, "duration=" + duration.String()}
	fields = append(fields, details...)
	log.Print(strings.Join(fields, " "))
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseDateQuery(value string) sql.NullTime {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullTime{}
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: parsed, Valid: true}
}

func parseBoolQuery(value string) sql.NullBool {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "true", "1", "yes":
		return sql.NullBool{Bool: true, Valid: true}
	case "false", "0", "no":
		return sql.NullBool{Bool: false, Valid: true}
	default:
		return sql.NullBool{}
	}
}
