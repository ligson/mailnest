package api

import (
	"context"
	"log"
	"net/http"

	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

// App 负责装配 HTTP 路由、存储层、邮件服务和 OAuth 服务。
type App struct {
	cfg         config.Config
	store       *storage.Store
	mailService *mail.Service
	oauth       *oauth.Service
	captchas    *captchaStore
}

func NewApp(cfg config.Config) (*App, error) {
	return NewAppWithFetcher(cfg, nil)
}

func NewAppWithFetcher(cfg config.Config, fetcher mail.Fetcher) (*App, error) {
	return NewAppWithDependencies(cfg, fetcher, oauth.NewMicrosoftExchanger(cfg.OAuth.Microsoft))
}

func NewAppWithDependencies(cfg config.Config, fetcher mail.Fetcher, exchanger oauth.MicrosoftExchanger) (*App, error) {
	store, err := storage.OpenWithOptions(storage.DatabaseOptions{
		Driver:       cfg.Database.Driver,
		DSN:          cfg.Database.DSN,
		Path:         cfg.Database.Path,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
	})
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
		captchas:    newCaptchaStore(),
	}, nil
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", a.handleHealth)
	mux.HandleFunc("GET /api/v1/auth/captcha", a.handleCaptcha)
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
	mux.Handle("POST /api/v1/messages/batch-actions", a.authMiddleware(http.HandlerFunc(a.handleMessageBatchAction)))
	mux.Handle("POST /api/v1/messages/batch-preview", a.authMiddleware(http.HandlerFunc(a.handleMessageBatchPreview)))
	mux.Handle("POST /api/v1/messages/send", a.authMiddleware(http.HandlerFunc(a.handleSendMessage)))
	mux.Handle("GET /api/v1/messages/{id}/compose-context", a.authMiddleware(http.HandlerFunc(a.handleMessageComposeContext)))
	mux.Handle("GET /api/v1/messages/{id}", a.authMiddleware(http.HandlerFunc(a.handleMessageDetail)))
	mux.Handle("POST /api/v1/messages/{id}/folder", a.authMiddleware(http.HandlerFunc(a.handleAssignMessageFolder)))
	mux.Handle("GET /api/v1/messages/{id}/attachments/{attachmentId}/content", a.authMiddleware(http.HandlerFunc(a.handleAttachmentContent)))
	mux.HandleFunc("GET /api/v1/messages/{id}/attachments/{attachmentId}/inline-content", a.handleInlineAttachmentContent)
	mux.Handle("GET /api/v1/attachments", a.authMiddleware(http.HandlerFunc(a.handleListAttachments)))
	mux.Handle("GET /api/v1/sync-jobs", a.authMiddleware(http.HandlerFunc(a.handleListSyncJobs)))
	mux.Handle("GET /api/v1/sync-jobs/{id}", a.authMiddleware(http.HandlerFunc(a.handleSyncJobDetail)))
	mux.Handle("GET /api/v1/sync-jobs/{id}/events", a.authMiddleware(http.HandlerFunc(a.handleSyncJobEvents)))
	mux.Handle("GET /api/v1/contacts", a.authMiddleware(http.HandlerFunc(a.handleListContacts)))
	mux.Handle("POST /api/v1/contacts", a.authMiddleware(http.HandlerFunc(a.handleCreateContact)))
	mux.Handle("PUT /api/v1/contacts/{id}", a.authMiddleware(http.HandlerFunc(a.handleUpdateContact)))
	mux.Handle("DELETE /api/v1/contacts/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteContact)))
	mux.Handle("GET /api/v1/mail-folders", a.authMiddleware(http.HandlerFunc(a.handleListMailFolders)))
	mux.Handle("POST /api/v1/mail-folders", a.authMiddleware(http.HandlerFunc(a.handleCreateMailFolder)))
	mux.Handle("PUT /api/v1/mail-folders/{id}", a.authMiddleware(http.HandlerFunc(a.handleUpdateMailFolder)))
	mux.Handle("DELETE /api/v1/mail-folders/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteMailFolder)))
	mux.Handle("GET /api/v1/mail-rules", a.authMiddleware(http.HandlerFunc(a.handleListMailRules)))
	mux.Handle("POST /api/v1/mail-rules", a.authMiddleware(http.HandlerFunc(a.handleCreateMailRule)))
	mux.Handle("PUT /api/v1/mail-rules/{id}", a.authMiddleware(http.HandlerFunc(a.handleUpdateMailRule)))
	mux.Handle("DELETE /api/v1/mail-rules/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteMailRule)))
	mux.Handle("POST /api/v1/mail-rules/apply", a.authMiddleware(http.HandlerFunc(a.handleApplyMailRules)))
	mux.Handle("POST /api/v1/mail-rules/preview", a.authMiddleware(http.HandlerFunc(a.handlePreviewMailRule)))
	mux.Handle("POST /api/v1/oauth/microsoft/start", a.authMiddleware(http.HandlerFunc(a.handleMicrosoftOAuthStart)))
	mux.Handle("POST /api/v1/oauth/microsoft/complete", a.authMiddleware(http.HandlerFunc(a.handleMicrosoftOAuthComplete)))
	mux.Handle("GET /api/v1/admin/users", a.adminMiddleware(http.HandlerFunc(a.handleAdminListUsers)))
	mux.Handle("PUT /api/v1/admin/users/{id}/enabled", a.adminMiddleware(http.HandlerFunc(a.handleAdminUpdateUserEnabled)))
	return mux
}

func (a *App) StartBackgroundTasks(ctx context.Context) {
	go func() {
		log.Printf("后台任务启动：修复历史邮件解析内容")
		if err := a.mailService.RepairStoredParsedMessages(); err != nil {
			log.Printf("后台任务失败：修复历史邮件解析内容失败 err=%v", err)
			return
		}
		log.Printf("后台任务完成：历史邮件解析内容修复结束")
	}()
	a.mailService.StartAutoSyncScheduler(ctx, mail.AutoSyncOptions{})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	response.OK(w, "服务正常", map[string]any{"status": "ok"})
}
