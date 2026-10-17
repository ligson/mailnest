package api

import (
	"net/http"

	"mailnest-be/internal/buildinfo"
	"mailnest-be/internal/response"
)

// 仅返回当前运行二进制的信息，避免页面版本取自配置或镜像标签造成误报。
func (a *App) handleAdminVersion(w http.ResponseWriter, r *http.Request) {
	response.OK(w, "获取成功", map[string]any{"backend": buildinfo.Current()})
}
