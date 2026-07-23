package api

import (
	"net/http"
	"strings"

	"mailnest-be/internal/response"
)

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
