package api

import (
	"net/http"
	"testing"

	"mailnest-be/internal/buildinfo"
)

func TestAdminVersionPermissionsAndEnvelope(t *testing.T) {
	router := newTestRouter(t, true)
	adminToken := registerTestUser(t, router, "version-admin", "version-admin@example.com")
	userToken := registerTestUser(t, router, "version-user", "version-user@example.com")
	for _, tc := range []struct {
		name, token string
		status      int
	}{
		{"未登录", "", http.StatusUnauthorized},
		{"普通用户", userToken, http.StatusForbidden},
		{"管理员", adminToken, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performRequest(router, http.MethodGet, "/api/v1/admin/version", "", tc.token)
			if resp.Code != tc.status {
				t.Fatalf("状态码错误：%d %s", resp.Code, resp.Body.String())
			}
			envelope := decodeEnvelope(t, resp.Body.Bytes())
			if envelope["httpCode"] != float64(tc.status) || envelope["success"] != (tc.status == http.StatusOK) {
				t.Fatalf("响应封装不一致：%#v", envelope)
			}
			if tc.status == http.StatusOK {
				data := envelope["data"].(map[string]any)
				backend := data["backend"].(map[string]any)
				want := buildinfo.Current()
				if len(data) != 1 || len(backend) != 3 || backend["version"] != want.Version || backend["commit"] != want.Commit || backend["buildTime"] != want.BuildTime {
					t.Fatalf("版本字段不正确：%#v", data)
				}
			}
		})
	}
}
