package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"mailnest-be/internal/mail"
	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

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

func (a *App) handleUpdateMailFolder(w http.ResponseWriter, r *http.Request) {
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
	folder, err := a.store.UpdateMailFolder(userID, folderID, storage.CreateMailFolderParams{
		UserID:    userID,
		Name:      req.Name,
		Color:     req.Color,
		SortOrder: req.SortOrder,
	})
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "文件夹不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusConflict, "文件夹名称已存在或更新失败")
		return
	}
	response.OK(w, "更新成功", mailFolderPayload(folder))
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
	} else if errors.Is(err, storage.ErrMailFolderHasRules) {
		response.Error(w, http.StatusConflict, "该文件夹已有规则关联，请先调整或删除相关规则")
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
	actionType := strings.ToLower(strings.TrimSpace(req.ActionType))
	switch actionType {
	case "mark_read", "star", "mark_spam":
	case "move_folder", "":
		actionType = "move_folder"
	default:
		response.Error(w, http.StatusBadRequest, "规则动作不支持")
		return storage.CreateMailRuleParams{}, false
	}
	var targetFolderID int64
	if actionType == "move_folder" {
		var err error
		targetFolderID, err = strconv.ParseInt(req.TargetFolderID, 10, 64)
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
		Priority:       req.Priority,
		StopOnMatch:    req.StopOnMatch,
		ActionType:     actionType,
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
	if scope != mail.RuleApplyScopeAll && scope != mail.RuleApplyScopeFiltered {
		scope = mail.RuleApplyScopeUnfiled
	}
	count, err := a.mailService.ApplyRules(userID, scope)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "应用规则失败")
		return
	}
	response.OK(w, "应用完成", map[string]any{"appliedCount": count})
}

func (a *App) handlePreviewMailRule(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req previewMailRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	params, ok := a.mailRuleParamsFromRequest(w, userID, createMailRuleRequest{
		Name:           valueOrFallback(req.Name, "规则预览"),
		Enabled:        true,
		MatchMode:      req.MatchMode,
		Priority:       req.Priority,
		StopOnMatch:    req.StopOnMatch,
		ActionType:     req.ActionType,
		TargetFolderID: req.TargetFolderID,
		SortOrder:      req.SortOrder,
		Conditions:     req.Conditions,
	})
	if !ok {
		return
	}
	rule := storage.MailRule{
		UserID:         userID,
		Name:           params.Name,
		Enabled:        true,
		MatchMode:      params.MatchMode,
		Priority:       params.Priority,
		StopOnMatch:    params.StopOnMatch,
		ActionType:     params.ActionType,
		TargetFolderID: params.TargetFolderID,
		SortOrder:      params.SortOrder,
		Conditions:     make([]storage.MailRuleCondition, 0, len(params.Conditions)),
	}
	for _, condition := range params.Conditions {
		rule.Conditions = append(rule.Conditions, storage.MailRuleCondition{
			Field:    condition.Field,
			Operator: condition.Operator,
			Value:    condition.Value,
		})
	}
	matchedCount, samples, err := a.mailService.PreviewRule(userID, rule, req.Limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "规则预览失败")
		return
	}
	items := make([]map[string]any, 0, len(samples))
	for _, sample := range samples {
		items = append(items, messageListPayload(sample))
	}
	response.OK(w, "获取成功", map[string]any{
		"matchedCount": matchedCount,
		"samples":      items,
	})
}
