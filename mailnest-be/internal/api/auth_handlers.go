package api

import (
	"errors"
	"net/http"
	"strings"

	"mailnest-be/internal/auth"
	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

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
