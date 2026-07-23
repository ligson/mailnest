package api

import (
	"fmt"
	"net/http"
	"testing"
)

func TestCaptchaRequiredAndOneTimeUse(t *testing.T) {
	router := newTestRouter(t, true)

	missingResp := performRequest(router, http.MethodPost, "/api/v1/auth/register", `{"username":"demo","email":"demo@example.com","password":"password123"}`, "")
	if missingResp.Code != http.StatusBadRequest {
		t.Fatalf("expected missing captcha status 400, got %d: %s", missingResp.Code, missingResp.Body.String())
	}

	invalidResp := performRequest(router, http.MethodPost, "/api/v1/auth/login", `{"account":"demo","password":"password123","captchaId":"missing","captchaAnswer":"ABCD"}`, "")
	if invalidResp.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid captcha status 400, got %d: %s", invalidResp.Code, invalidResp.Body.String())
	}

	captchaID, captchaAnswer := captchaChallenge(t, router)
	registerBody := fmt.Sprintf(`{"username":"demo","email":"demo@example.com","password":"password123","captchaId":%q,"captchaAnswer":%q}`, captchaID, captchaAnswer)
	registerResp := performRequest(router, http.MethodPost, "/api/v1/auth/register", registerBody, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("expected register status 201, got %d: %s", registerResp.Code, registerResp.Body.String())
	}

	reusedBody := fmt.Sprintf(`{"account":"demo","password":"password123","captchaId":%q,"captchaAnswer":%q}`, captchaID, captchaAnswer)
	reusedResp := performRequest(router, http.MethodPost, "/api/v1/auth/login", reusedBody, "")
	if reusedResp.Code != http.StatusBadRequest {
		t.Fatalf("expected reused captcha status 400, got %d: %s", reusedResp.Code, reusedResp.Body.String())
	}
}

func TestAdminCanListAndDisableUsers(t *testing.T) {
	router := newTestRouter(t, true)
	adminToken := registerTestUser(t, router, "admin-user", "admin@example.com")
	userToken := registerTestUser(t, router, "normal-user", "normal@example.com")

	adminMeResp := performRequest(router, http.MethodGet, "/api/v1/auth/me", "", adminToken)
	if adminMeResp.Code != http.StatusOK {
		t.Fatalf("expected admin me status 200, got %d: %s", adminMeResp.Code, adminMeResp.Body.String())
	}
	adminMe := decodeEnvelope(t, adminMeResp.Body.Bytes())["data"].(map[string]any)
	if adminMe["isAdmin"] != true {
		t.Fatalf("expected first user to be admin, got %#v", adminMe)
	}

	userMeResp := performRequest(router, http.MethodGet, "/api/v1/auth/me", "", userToken)
	if userMeResp.Code != http.StatusOK {
		t.Fatalf("expected user me status 200, got %d: %s", userMeResp.Code, userMeResp.Body.String())
	}
	userMe := decodeEnvelope(t, userMeResp.Body.Bytes())["data"].(map[string]any)
	if userMe["isAdmin"] != false {
		t.Fatalf("expected second user not to be admin, got %#v", userMe)
	}

	forbiddenResp := performRequest(router, http.MethodGet, "/api/v1/admin/users", "", userToken)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin status 403, got %d: %s", forbiddenResp.Code, forbiddenResp.Body.String())
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/admin/users", "", adminToken)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected admin list status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	items := decodeEnvelope(t, listResp.Body.Bytes())["data"].(map[string]any)["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 users, got %#v", items)
	}
	normalUserID := findAdminUserID(t, items, "normal-user")
	normalItem := findAdminUserItem(t, items, "normal-user")
	if normalItem["attachmentBytes"] != float64(0) {
		t.Fatalf("expected storage usage field attachmentBytes, got %#v", normalItem)
	}

	selfDisableResp := performRequest(router, http.MethodPut, "/api/v1/admin/users/"+adminMe["id"].(string)+"/enabled", `{"enabled":false}`, adminToken)
	if selfDisableResp.Code != http.StatusBadRequest {
		t.Fatalf("expected self-disable status 400, got %d: %s", selfDisableResp.Code, selfDisableResp.Body.String())
	}

	disableResp := performRequest(router, http.MethodPut, "/api/v1/admin/users/"+normalUserID+"/enabled", `{"enabled":false}`, adminToken)
	if disableResp.Code != http.StatusOK {
		t.Fatalf("expected disable status 200, got %d: %s", disableResp.Code, disableResp.Body.String())
	}
	disabledData := decodeEnvelope(t, disableResp.Body.Bytes())["data"].(map[string]any)
	if disabledData["enabled"] != false {
		t.Fatalf("expected disabled payload, got %#v", disabledData)
	}

	disabledMeResp := performRequest(router, http.MethodGet, "/api/v1/auth/me", "", userToken)
	if disabledMeResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected disabled token status 401, got %d: %s", disabledMeResp.Code, disabledMeResp.Body.String())
	}

	disabledLoginResp := loginTestUser(t, router, "normal-user", "password123")
	if disabledLoginResp.Code != http.StatusForbidden {
		t.Fatalf("expected disabled login status 403, got %d: %s", disabledLoginResp.Code, disabledLoginResp.Body.String())
	}
}

func findAdminUserID(t *testing.T, items []any, username string) string {
	t.Helper()
	item := findAdminUserItem(t, items, username)
	id, ok := item["id"].(string)
	if !ok || id == "" {
		t.Fatalf("expected user id for %s, got %#v", username, item["id"])
	}
	return id
}

func findAdminUserItem(t *testing.T, items []any, username string) map[string]any {
	t.Helper()
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			t.Fatalf("expected user item object, got %#v", rawItem)
		}
		if item["username"] == username {
			return item
		}
	}
	t.Fatalf("expected user %s in %#v", username, items)
	return nil
}
