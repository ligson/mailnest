package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"
)

func TestAuthFlowRegisterLoginAndMe(t *testing.T) {
	router := newTestRouter(t, true)

	captchaID, captchaAnswer := captchaChallenge(t, router)
	registerBody := `{"username":"demo","email":"demo@example.com","password":"password123","captchaId":"` + captchaID + `","captchaAnswer":"` + captchaAnswer + `"}`
	registerResp := performRequest(router, http.MethodPost, "/api/v1/auth/register", registerBody, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("expected register status 201, got %d: %s", registerResp.Code, registerResp.Body.String())
	}

	var registerEnvelope map[string]any
	if err := json.Unmarshal(registerResp.Body.Bytes(), &registerEnvelope); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}
	if registerEnvelope["success"] != true {
		t.Fatalf("expected register success, got %#v", registerEnvelope)
	}
	token := nestedString(t, registerEnvelope, "data", "token")
	if token == "" {
		t.Fatal("expected token in register response")
	}

	loginResp := loginTestUser(t, router, "demo", "password123")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", loginResp.Code, loginResp.Body.String())
	}

	meResp := performRequest(router, http.MethodGet, "/api/v1/auth/me", "", token)
	if meResp.Code != http.StatusOK {
		t.Fatalf("expected me status 200, got %d: %s", meResp.Code, meResp.Body.String())
	}

	var meEnvelope map[string]any
	if err := json.Unmarshal(meResp.Body.Bytes(), &meEnvelope); err != nil {
		t.Fatalf("unmarshal me response: %v", err)
	}
	if nestedString(t, meEnvelope, "data", "username") != "demo" {
		t.Fatalf("expected username demo, got %#v", meEnvelope)
	}
	if nestedString(t, meEnvelope, "data", "uiTheme") != "forest" {
		t.Fatalf("expected default uiTheme forest, got %#v", meEnvelope)
	}
	if meEnvelope["data"].(map[string]any)["isAdmin"] != true {
		t.Fatalf("expected first registered user to be admin, got %#v", meEnvelope)
	}
	if meEnvelope["data"].(map[string]any)["enabled"] != true {
		t.Fatalf("expected first registered user enabled, got %#v", meEnvelope)
	}
}

func TestRegisterRespectsConfigSwitch(t *testing.T) {
	router := newTestRouter(t, false)

	resp := performRequest(router, http.MethodPost, "/api/v1/auth/register", `{"username":"demo","email":"demo@example.com","password":"password123"}`, "")

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", resp.Code, resp.Body.String())
	}

	var envelope map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope["success"] != false {
		t.Fatalf("expected success false, got %#v", envelope)
	}
	if envelope["httpCode"].(float64) != http.StatusForbidden {
		t.Fatalf("expected httpCode 403, got %#v", envelope["httpCode"])
	}
}

func TestChangePasswordRequiresLoginAndUpdatesLoginCredential(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "change-password", "change-password@example.com")

	unauthorizedResp := performRequest(router, http.MethodPost, "/api/v1/auth/change-password", `{"currentPassword":"password123","newPassword":"new-password-123","confirmPassword":"new-password-123"}`, "")
	if unauthorizedResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status 401, got %d: %s", unauthorizedResp.Code, unauthorizedResp.Body.String())
	}

	wrongCurrentResp := performRequest(router, http.MethodPost, "/api/v1/auth/change-password", `{"currentPassword":"wrong-password","newPassword":"new-password-123","confirmPassword":"new-password-123"}`, token)
	if wrongCurrentResp.Code != http.StatusBadRequest {
		t.Fatalf("expected wrong current password status 400, got %d: %s", wrongCurrentResp.Code, wrongCurrentResp.Body.String())
	}

	samePasswordResp := performRequest(router, http.MethodPost, "/api/v1/auth/change-password", `{"currentPassword":"password123","newPassword":"password123","confirmPassword":"password123"}`, token)
	if samePasswordResp.Code != http.StatusBadRequest {
		t.Fatalf("expected same password status 400, got %d: %s", samePasswordResp.Code, samePasswordResp.Body.String())
	}

	changeResp := performRequest(router, http.MethodPost, "/api/v1/auth/change-password", `{"currentPassword":"password123","newPassword":"new-password-123","confirmPassword":"new-password-123"}`, token)
	if changeResp.Code != http.StatusOK {
		t.Fatalf("expected change password status 200, got %d: %s", changeResp.Code, changeResp.Body.String())
	}
	if decodeEnvelope(t, changeResp.Body.Bytes())["success"] != true {
		t.Fatalf("expected change password success, got %s", changeResp.Body.String())
	}

	oldLoginResp := loginTestUser(t, router, "change-password", "password123")
	if oldLoginResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password login status 401, got %d: %s", oldLoginResp.Code, oldLoginResp.Body.String())
	}

	newLoginResp := loginTestUser(t, router, "change-password", "new-password-123")
	if newLoginResp.Code != http.StatusOK {
		t.Fatalf("expected new password login status 200, got %d: %s", newLoginResp.Code, newLoginResp.Body.String())
	}
}

func TestProfileCanBeUpdatedAndAvatarCanBeUploaded(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "profile-user", "profile-user@example.com")

	updateResp := performRequest(router, http.MethodPut, "/api/v1/profile", `{"nickname":"信匣用户","bio":"用 Mail Nest 管理邮件","uiTheme":"grape"}`, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update profile status 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}
	data := decodeEnvelope(t, updateResp.Body.Bytes())["data"].(map[string]any)
	if data["nickname"] != "信匣用户" || data["bio"] != "用 Mail Nest 管理邮件" || data["uiTheme"] != "grape" {
		t.Fatalf("expected updated profile data, got %#v", data)
	}

	meResp := performRequest(router, http.MethodGet, "/api/v1/auth/me", "", token)
	if meResp.Code != http.StatusOK {
		t.Fatalf("expected me status 200, got %d: %s", meResp.Code, meResp.Body.String())
	}
	if nestedString(t, decodeEnvelope(t, meResp.Body.Bytes()), "data", "nickname") != "信匣用户" {
		t.Fatalf("expected me payload to include nickname, got %s", meResp.Body.String())
	}
	if nestedString(t, decodeEnvelope(t, meResp.Body.Bytes()), "data", "uiTheme") != "grape" {
		t.Fatalf("expected me payload to include uiTheme, got %s", meResp.Body.String())
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatalf("create avatar form file: %v", err)
	}
	if _, err := part.Write([]byte("fake-png-bytes")); err != nil {
		t.Fatalf("write avatar: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	avatarResp := performMultipartRequest(router, http.MethodPost, "/api/v1/profile/avatar", body.Bytes(), writer.FormDataContentType(), token)
	if avatarResp.Code != http.StatusOK {
		t.Fatalf("expected upload avatar status 200, got %d: %s", avatarResp.Code, avatarResp.Body.String())
	}
	avatarURL := nestedString(t, decodeEnvelope(t, avatarResp.Body.Bytes()), "data", "avatarUrl")
	if avatarURL == "" {
		t.Fatalf("expected avatarUrl, got %s", avatarResp.Body.String())
	}

	contentResp := performRequest(router, http.MethodGet, avatarURL, "", token)
	if contentResp.Code != http.StatusOK {
		t.Fatalf("expected avatar content status 200, got %d: %s", contentResp.Code, contentResp.Body.String())
	}
	if contentResp.Body.String() != "fake-png-bytes" {
		t.Fatalf("expected avatar bytes, got %q", contentResp.Body.String())
	}
}
