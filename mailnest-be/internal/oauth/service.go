package oauth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"sync"

	"mailnest-be/internal/crypto"
	"mailnest-be/internal/storage"
)

type Service struct {
	store            *storage.Store
	exchanger        MicrosoftExchanger
	credentialSecret string
	redirectURL      string
	states           map[string]int64
	mu               sync.Mutex
}

type StartResult struct {
	State   string
	AuthURL string
}

func NewService(store *storage.Store, exchanger MicrosoftExchanger, credentialSecret, redirectURL string) *Service {
	return &Service{
		store:            store,
		exchanger:        exchanger,
		credentialSecret: credentialSecret,
		redirectURL:      redirectURL,
		states:           make(map[string]int64),
	}
}

func (s *Service) StartMicrosoft(userID int64) (StartResult, error) {
	state, err := randomState()
	if err != nil {
		return StartResult{}, err
	}
	s.mu.Lock()
	s.states[state] = userID
	s.mu.Unlock()
	return StartResult{
		State:   state,
		AuthURL: s.exchanger.AuthCodeURL(state, s.redirectURL),
	}, nil
}

func (s *Service) CompleteMicrosoft(userID int64, code, state string) (storage.MailAccount, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(state) == "" {
		return storage.MailAccount{}, errors.New("授权参数不完整")
	}
	if !s.consumeState(userID, state) {
		return storage.MailAccount{}, errors.New("授权状态无效或已过期")
	}
	token, account, err := s.exchanger.Exchange(code, s.redirectURL)
	if err != nil {
		return storage.MailAccount{}, err
	}
	encryptedAccess, err := crypto.EncryptString(token.AccessToken, s.credentialSecret)
	if err != nil {
		return storage.MailAccount{}, err
	}
	encryptedRefresh, err := crypto.EncryptString(token.RefreshToken, s.credentialSecret)
	if err != nil {
		return storage.MailAccount{}, err
	}

	return s.store.CreateMailAccount(storage.MailAccount{
		UserID:              userID,
		Provider:            "microsoft",
		AuthType:            "oauth2",
		DisplayName:         "Microsoft 邮箱",
		Email:               account.Email,
		IMAPHost:            "outlook.office365.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        account.Email,
		IMAPPasswordEncoded: "",
		OAuthAccessToken:    sql.NullString{String: encryptedAccess, Valid: true},
		OAuthRefreshToken:   sql.NullString{String: encryptedRefresh, Valid: token.RefreshToken != ""},
		OAuthExpiresAt:      sql.NullTime{Time: token.ExpiresAt, Valid: !token.ExpiresAt.IsZero()},
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
}

func (s *Service) consumeState(userID int64, state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expected, ok := s.states[state]
	if ok {
		delete(s.states, state)
	}
	return ok && expected == userID
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
