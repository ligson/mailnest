package api

import (
	"database/sql"
	"net/http"
	netmail "net/mail"
	"strings"
	"time"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func contactParamsFromRequest(w http.ResponseWriter, userID int64, req contactRequest) (storage.CreateContactParams, bool) {
	email, displayFromEmail, ok := normalizeContactEmail(w, req.Email)
	if !ok {
		return storage.CreateContactParams{}, false
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		req.DisplayName = displayFromEmail
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Company = strings.TrimSpace(req.Company)
	req.Notes = strings.TrimSpace(req.Notes)
	if len([]rune(req.DisplayName)) > 80 {
		response.Error(w, http.StatusBadRequest, "联系人姓名不能超过 80 个字符")
		return storage.CreateContactParams{}, false
	}
	if len([]rune(req.Nickname)) > 80 {
		response.Error(w, http.StatusBadRequest, "联系人昵称不能超过 80 个字符")
		return storage.CreateContactParams{}, false
	}
	if len([]rune(req.Phone)) > 40 {
		response.Error(w, http.StatusBadRequest, "联系电话不能超过 40 个字符")
		return storage.CreateContactParams{}, false
	}
	if len([]rune(req.Company)) > 120 {
		response.Error(w, http.StatusBadRequest, "公司不能超过 120 个字符")
		return storage.CreateContactParams{}, false
	}
	if len([]rune(req.Notes)) > 500 {
		response.Error(w, http.StatusBadRequest, "备注不能超过 500 个字符")
		return storage.CreateContactParams{}, false
	}
	return storage.CreateContactParams{
		UserID:      userID,
		Email:       email,
		DisplayName: req.DisplayName,
		Nickname:    req.Nickname,
		Phone:       req.Phone,
		Company:     req.Company,
		Notes:       req.Notes,
		Source:      "manual",
		SeenAt:      sql.NullTime{Time: time.Now(), Valid: true},
	}, true
}

func normalizeContactEmail(w http.ResponseWriter, value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		response.Error(w, http.StatusBadRequest, "联系人邮箱不能为空")
		return "", "", false
	}
	address, err := netmail.ParseAddress(value)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		response.Error(w, http.StatusBadRequest, "联系人邮箱格式不正确")
		return "", "", false
	}
	email := strings.ToLower(strings.TrimSpace(address.Address))
	if len([]rune(email)) > 254 {
		response.Error(w, http.StatusBadRequest, "联系人邮箱过长")
		return "", "", false
	}
	return email, strings.TrimSpace(address.Name), true
}
