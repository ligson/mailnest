package api

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
)

func TestSearchStatusAndRebuildAuthorization(t *testing.T) {
	cfg := config.Default()
	cfg.App.DataDir = t.TempDir()
	cfg.Database.Path = filepath.Join(cfg.App.DataDir, "mail.db")
	cfg.Search.Engine = "bleve"
	app, err := NewAppWithDependencies(cfg, &mail.FakeFetcher{}, &oauth.FakeMicrosoftExchanger{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	router := app.Routes()
	admin := registerTestUser(t, router, "admin-search", "admin-search@example.com")
	normal := registerTestUser(t, router, "normal-search", "normal-search@example.com")
	for _, tc := range []struct {
		method, path, token string
		code                int
	}{{"GET", "/api/v1/search/status", "", 401}, {"POST", "/api/v1/admin/search/rebuild", "", 401}, {"POST", "/api/v1/admin/search/rebuild", normal, 403}, {"POST", "/api/v1/admin/search/rebuild", admin, 409}} {
		resp := performRequest(router, tc.method, tc.path, "", tc.token)
		envelope := decodeEnvelope(t, resp.Body.Bytes())
		if resp.Code != tc.code || envelope["httpCode"] != float64(tc.code) || envelope["success"] != false {
			t.Fatalf("response: %d %s", resp.Code, resp.Body.String())
		}
	}
	resp := performRequest(router, http.MethodGet, "/api/v1/search/status", "", normal)
	data := decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)
	if resp.Code != 200 || data["shadowChecks"] != nil || data["shadowMissing"] != nil || data["total"] != float64(0) {
		t.Fatalf("ordinary user status leaked: %s", resp.Body.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.searchIndex.Start(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, err := app.searchIndex.Status(ctx, 1, true)
		if err != nil {
			t.Fatal(err)
		}
		if status.Available && status.Phase == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("initial build timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	resp = performRequest(router, http.MethodPost, "/api/v1/admin/search/rebuild", "", admin)
	envelope := decodeEnvelope(t, resp.Body.Bytes())
	if resp.Code != 202 || envelope["httpCode"] != float64(202) || envelope["success"] != true {
		t.Fatalf("rebuild: %d %s", resp.Code, resp.Body.String())
	}
}

func TestSearchDatabaseFallbackStatus(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "fallback-user", "fallback@example.com")
	resp := performRequest(router, "GET", "/api/v1/search/status", "", token)
	data := decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)
	if data["engine"] != "database" || data["phase"] != "disabled" {
		t.Fatalf("fallback status: %s", resp.Body.String())
	}
	if resp := performRequest(router, "POST", "/api/v1/admin/search/rebuild", "", token); resp.Code != 409 {
		t.Fatalf("disabled rebuild: %d", resp.Code)
	}
}
