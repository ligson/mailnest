package api

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

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
	req.UITheme = normalizeUITheme(req.UITheme)
	if len([]rune(req.Nickname)) > 40 {
		response.Error(w, http.StatusBadRequest, "昵称不能超过 40 个字符")
		return
	}
	if len([]rune(req.Bio)) > 200 {
		response.Error(w, http.StatusBadRequest, "个人描述不能超过 200 个字符")
		return
	}

	user, err := a.store.UpdateUserProfile(userID, req.Nickname, req.Bio, req.UITheme)
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
