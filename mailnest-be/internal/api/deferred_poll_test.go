package api

import (
	"context"
	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/storage"
	"path/filepath"
	"testing"
	"time"
)

type delayedCountProvider struct{}

func (delayedCountProvider) Candidates(ctx context.Context, _ storage.MailSearchTextQuery) (storage.MailSearchCandidates, error) {
	select {
	case <-time.After(400 * time.Millisecond):
		return storage.MailSearchCandidates{}, nil
	case <-ctx.Done():
		return storage.MailSearchCandidates{}, ctx.Err()
	}
}
func (delayedCountProvider) ObserveShadow(int64, []int64, storage.MailSearchCandidates) {}
func (delayedCountProvider) Mode() string                                               { return "slow-test" }

func TestCountPollingEnvelope(t *testing.T) {
	cfg := config.Default()
	cfg.App.DataDir = t.TempDir()
	cfg.Database.Path = filepath.Join(cfg.App.DataDir, "mail.db")
	app, err := NewAppWithDependencies(cfg, &mail.FakeFetcher{}, &oauth.FakeMicrosoftExchanger{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	router := app.Routes()
	token := registerTestUser(t, router, "poll-user", "poll@example.com")
	app.store.SetMailSearchProvider(delayedCountProvider{})
	for _, kind := range []string{"messages", "threads"} {
		path := "/api/v1/" + kind + "/count?keyword=" + kind + "&deferred=true"
		start := time.Now()
		resp := performRequest(router, "GET", path, "", token)
		e := decodeEnvelope(t, resp.Body.Bytes())
		data := e["data"].(map[string]any)
		if resp.Code != 202 || e["httpCode"] != float64(202) || e["success"] != true || data["total"] != nil || data["totalPending"] != true || time.Since(start) > time.Second {
			t.Fatalf("等待响应错误 %s", resp.Body.String())
		}
		time.Sleep(450 * time.Millisecond)
		resp = performRequest(router, "GET", path, "", token)
		data = decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)
		if resp.Code != 200 || data["total"] != float64(0) {
			t.Fatalf("轮询未获得精确结果 %s", resp.Body.String())
		}
	}
}
