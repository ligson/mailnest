package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"mailnest-be/internal/crypto"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

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

	log.Printf("邮箱账号创建成功 userID=%d accountID=%d imapHost=%s sentFolder=%s enabled=%t", userID, account.ID, account.IMAPHost, account.SentFolder, account.Enabled)
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

	log.Printf("邮箱账号更新成功 userID=%d accountID=%d imapHost=%s sentFolder=%s enabled=%t cleanupEnabled=%t", userID, account.ID, account.IMAPHost, account.SentFolder, account.Enabled, account.CleanupEnabled)
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

	log.Printf("邮箱账号删除成功 userID=%d accountID=%d", userID, id)
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

	log.Printf("用户手动收取完成 userID=%d accountID=%d jobID=%d new=%d warnings=%d", userID, accountID, result.JobID, result.NewMessageCount, len(result.Warnings))
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

	log.Printf("用户启动全量同步 userID=%d accountID=%d status=%s", userID, accountID, status.Status)
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

	log.Printf("用户停止全量同步 userID=%d accountID=%d status=%s processed=%d total=%d", userID, accountID, status.Status, status.Processed, status.Total)
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
