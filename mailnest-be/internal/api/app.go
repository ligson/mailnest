package api

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
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
)

type App struct {
	cfg         config.Config
	store       *storage.Store
	mailService *mail.Service
	oauth       *oauth.Service
}

type contextKey string

const userIDKey contextKey = "userID"

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

	return &App{
		cfg:         cfg,
		store:       store,
		mailService: mail.NewService(store, fetcher, exchanger, cfg.App.DataDir, cfg.App.CredentialSecret),
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
	mux.Handle("GET /api/v1/mail-accounts", a.authMiddleware(http.HandlerFunc(a.handleListMailAccounts)))
	mux.Handle("POST /api/v1/mail-accounts", a.authMiddleware(http.HandlerFunc(a.handleCreateMailAccount)))
	mux.Handle("PUT /api/v1/mail-accounts/{id}", a.authMiddleware(http.HandlerFunc(a.handleUpdateMailAccount)))
	mux.Handle("DELETE /api/v1/mail-accounts/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteMailAccount)))
	mux.Handle("POST /api/v1/mail-accounts/{id}/test-connection", a.authMiddleware(http.HandlerFunc(a.handleTestMailAccountConnection)))
	mux.Handle("POST /api/v1/mail-accounts/{id}/sync", a.authMiddleware(http.HandlerFunc(a.handleSyncMailAccount)))
	mux.Handle("GET /api/v1/messages", a.authMiddleware(http.HandlerFunc(a.handleListMessages)))
	mux.Handle("GET /api/v1/messages/{id}", a.authMiddleware(http.HandlerFunc(a.handleMessageDetail)))
	mux.Handle("POST /api/v1/messages/{id}/folder", a.authMiddleware(http.HandlerFunc(a.handleAssignMessageFolder)))
	mux.Handle("GET /api/v1/messages/{id}/attachments/{attachmentId}/content", a.authMiddleware(http.HandlerFunc(a.handleAttachmentContent)))
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

type createMailAccountRequest struct {
	DisplayName         string `json:"displayName"`
	Email               string `json:"email"`
	IMAPHost            string `json:"imapHost"`
	IMAPPort            int    `json:"imapPort"`
	IMAPTLS             bool   `json:"imapTls"`
	IMAPUsername        string `json:"imapUsername"`
	IMAPPassword        string `json:"imapPassword"`
	PollIntervalMinutes int    `json:"pollIntervalMinutes"`
	Enabled             bool   `json:"enabled"`
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

type assignMessageFolderRequest struct {
	FolderID string `json:"folderId"`
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
	if req.DisplayName == "" || req.Email == "" || req.IMAPHost == "" || req.IMAPUsername == "" || req.IMAPPassword == "" || req.IMAPPort <= 0 {
		response.Error(w, http.StatusBadRequest, "邮箱账号配置不完整")
		return
	}
	if req.PollIntervalMinutes <= 0 {
		req.PollIntervalMinutes = 10
	}

	encryptedPassword, err := crypto.EncryptString(req.IMAPPassword, a.cfg.App.CredentialSecret)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "邮箱凭据加密失败")
		return
	}

	account, err := a.store.CreateMailAccount(storage.MailAccount{
		UserID:              userID,
		DisplayName:         req.DisplayName,
		Email:               req.Email,
		IMAPHost:            req.IMAPHost,
		IMAPPort:            req.IMAPPort,
		IMAPTLS:             req.IMAPTLS,
		IMAPUsername:        req.IMAPUsername,
		IMAPPasswordEncoded: encryptedPassword,
		PollIntervalMinutes: req.PollIntervalMinutes,
		Enabled:             req.Enabled,
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
	if req.DisplayName == "" || req.Email == "" || req.IMAPHost == "" || req.IMAPUsername == "" || req.IMAPPort <= 0 {
		response.Error(w, http.StatusBadRequest, "邮箱账号配置不完整")
		return
	}
	if req.PollIntervalMinutes <= 0 {
		req.PollIntervalMinutes = 10
	}

	encryptedPassword := current.IMAPPasswordEncoded
	if strings.TrimSpace(req.IMAPPassword) != "" {
		encryptedPassword, err = crypto.EncryptString(req.IMAPPassword, a.cfg.App.CredentialSecret)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "邮箱凭据加密失败")
			return
		}
	}

	current.DisplayName = req.DisplayName
	current.Email = req.Email
	current.IMAPHost = req.IMAPHost
	current.IMAPPort = req.IMAPPort
	current.IMAPTLS = req.IMAPTLS
	current.IMAPUsername = req.IMAPUsername
	current.IMAPPasswordEncoded = encryptedPassword
	current.PollIntervalMinutes = req.PollIntervalMinutes
	current.Enabled = req.Enabled

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
	if err != nil {
		response.Error(w, http.StatusBadRequest, "收取失败："+err.Error())
		return
	}

	response.OK(w, "收取完成", map[string]any{
		"jobId":           strconv.FormatInt(result.JobID, 10),
		"newMessageCount": result.NewMessageCount,
	})
}

func (a *App) handleListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

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
		DateFrom:       parseDateQuery(r.URL.Query().Get("dateFrom")),
		DateTo:         parseDateQuery(r.URL.Query().Get("dateTo")),
		HasAttachments: parseBoolQuery(r.URL.Query().Get("hasAttachments")),
		Limit:          pageSize,
		Offset:         offset,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件列表失败")
		return
	}

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
}

func (a *App) handleMessageDetail(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}

	message, err := a.store.FindMailMessageByID(userID, id)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件详情失败")
		return
	}

	payload := messageListPayload(message)
	attachments, err := a.store.ListMailAttachments(userID, message.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件附件失败")
		return
	}
	payload["textBody"] = readOptionalFile(message.TextBodyPath)
	payload["htmlBody"] = rewriteInlineCIDImages(readOptionalFile(message.HTMLBodyPath), attachments)
	payload["cc"] = splitAddressField(message.CCAddrs)
	payload["folder"] = message.Folder
	payload["messageId"] = nullableString(message.MessageID)
	payload["attachments"] = attachmentPayloads(message.ID, attachments)

	response.OK(w, "获取成功", payload)
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

func userPayload(user storage.User) map[string]any {
	return map[string]any{
		"id":       strconv.FormatInt(user.ID, 10),
		"username": user.Username,
		"email":    user.Email,
	}
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
		"id":                  strconv.FormatInt(account.ID, 10),
		"provider":            account.Provider,
		"authType":            account.AuthType,
		"displayName":         account.DisplayName,
		"email":               account.Email,
		"imapHost":            account.IMAPHost,
		"imapPort":            account.IMAPPort,
		"imapTls":             account.IMAPTLS,
		"imapUsername":        account.IMAPUsername,
		"pollIntervalMinutes": account.PollIntervalMinutes,
		"enabled":             account.Enabled,
		"lastSyncAt":          lastSyncAt,
		"lastSyncStatus":      lastSyncStatus,
		"lastSyncError":       lastSyncError,
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

func attachmentPayloads(messageID int64, attachments []storage.MailAttachment) []map[string]any {
	items := make([]map[string]any, 0, len(attachments))
	for _, attachment := range attachments {
		items = append(items, map[string]any{
			"id":          strconv.FormatInt(attachment.ID, 10),
			"messageId":   strconv.FormatInt(messageID, 10),
			"filename":    attachment.Filename,
			"contentType": nullableString(attachment.ContentType),
			"contentId":   nullableString(attachment.ContentID),
			"inline":      attachment.Inline,
			"size":        attachment.Size,
			"downloadUrl": fmt.Sprintf("/api/v1/messages/%d/attachments/%d/content", messageID, attachment.ID),
		})
	}
	return items
}

func rewriteInlineCIDImages(htmlBody string, attachments []storage.MailAttachment) string {
	if strings.TrimSpace(htmlBody) == "" || len(attachments) == 0 {
		return htmlBody
	}
	result := htmlBody
	for _, attachment := range attachments {
		if !attachment.Inline || !attachment.ContentID.Valid || strings.TrimSpace(attachment.ContentID.String) == "" {
			continue
		}
		contentType := "application/octet-stream"
		if attachment.ContentType.Valid && strings.TrimSpace(attachment.ContentType.String) != "" {
			contentType = attachment.ContentType.String
		}
		data, err := os.ReadFile(attachment.FilePath)
		if err != nil {
			continue
		}
		dataURL := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
		contentID := strings.TrimSpace(attachment.ContentID.String)
		for _, cid := range []string{"cid:" + contentID, "cid:<" + contentID + ">"} {
			result = strings.ReplaceAll(result, cid, dataURL)
		}
	}
	return result
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
