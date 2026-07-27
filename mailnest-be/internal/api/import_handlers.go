package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleImportEMLMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	raw, filename, err := readImportEMLFile(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	accountID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("accountId")), 10, 64)
	if err != nil || accountID <= 0 {
		response.Error(w, http.StatusBadRequest, "请选择导入邮箱账号")
		return
	}
	folder := strings.TrimSpace(r.FormValue("folder"))
	if folder == "" {
		folder = "INBOX"
	}
	result, err := a.mailService.ImportEML(userID, accountID, folder, raw)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "导入 EML 失败："+err.Error())
		return
	}
	log.Printf("EML 邮件导入完成 userID=%d accountID=%d messageID=%d inserted=%t filename=%s", userID, accountID, result.Message.ID, result.Inserted, filename)
	response.OK(w, "导入成功", map[string]any{
		"inserted": result.Inserted,
		"message":  messageListPayload(result.Message),
	})
}
