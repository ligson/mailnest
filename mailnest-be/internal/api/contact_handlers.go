package api

import (
	"database/sql"
	"errors"
	"net/http"
	netmail "net/mail"
	"strconv"
	"strings"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleResolveContacts(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUserID(r)
	var req struct {
		Addresses []string `json:"addresses"`
	}
	if err := decodeJSON(r, &req); err != nil || len(req.Addresses) > 256 {
		response.Error(w, http.StatusBadRequest, "每次最多解析 256 个联系人地址")
		return
	}
	keys := []string{}
	for _, raw := range req.Addresses {
		raw = strings.TrimSpace(raw)
		if address, err := netmail.ParseAddress(raw); err == nil {
			raw = address.Address
		}
		if len(raw) > 254 {
			response.Error(w, http.StatusBadRequest, "联系人地址过长")
			return
		}
		keys = append(keys, raw)
	}
	result, err := a.store.ResolveContacts(r.Context(), userID, keys)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		response.Error(w, http.StatusInternalServerError, "解析联系人失败")
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, contact := range result.Items {
		items = append(items, contactPayload(contact))
	}
	response.OK(w, "", map[string]any{"items": items, "ambiguousPrefixes": result.AmbiguousPrefixes, "revision": a.store.MailReadVersion()})
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
