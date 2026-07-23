package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
)

func newTestRouter(t *testing.T, allowRegistration bool) http.Handler {
	return newTestRouterWithFetcher(t, allowRegistration, &mail.FakeFetcher{})
}

func newTestRouterWithFetcher(t *testing.T, allowRegistration bool, fetcher mail.Fetcher) http.Handler {
	return newTestRouterWithDependencies(t, allowRegistration, fetcher, &oauth.FakeMicrosoftExchanger{})
}

func newTestRouterWithDependencies(t *testing.T, allowRegistration bool, fetcher mail.Fetcher, exchanger oauth.MicrosoftExchanger) http.Handler {
	return newTestRouterWithDependenciesAndConfig(t, allowRegistration, fetcher, exchanger, nil)
}

func newTestRouterWithDependenciesAndConfig(t *testing.T, allowRegistration bool, fetcher mail.Fetcher, exchanger oauth.MicrosoftExchanger, configure func(*config.Config)) http.Handler {
	t.Helper()

	tempDir := t.TempDir()
	cfg := config.Default()
	cfg.App.DataDir = tempDir
	cfg.App.AllowRegistration = allowRegistration
	cfg.App.JWTSecret = "test-jwt-secret"
	cfg.Database.Path = filepath.Join(tempDir, "mailnest.db")
	cfg.OAuth.Microsoft.ClientID = "client-id"
	cfg.OAuth.Microsoft.ClientSecret = "client-secret"
	cfg.OAuth.Microsoft.RedirectURL = "http://127.0.0.1:5173/oauth/microsoft/callback"
	if configure != nil {
		configure(&cfg)
	}

	app, err := NewAppWithDependencies(cfg, fetcher, exchanger)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	return app.Routes()
}

type capturingFetcher struct {
	LastAccount mail.AccountConfig
	Messages    []mail.FetchedMessage
}

func (f *capturingFetcher) TestConnection(account mail.AccountConfig) error {
	f.LastAccount = account
	return nil
}

func (f *capturingFetcher) ListFolders(account mail.AccountConfig) ([]mail.FolderInfo, error) {
	f.LastAccount = account
	return []mail.FolderInfo{{Name: "INBOX"}, {Name: "Sent", Attributes: []string{"\\Sent"}}}, nil
}

func (f *capturingFetcher) FetchInbox(account mail.AccountConfig) ([]mail.FetchedMessage, error) {
	return f.FetchFolder(account)
}

func (f *capturingFetcher) FetchFolder(account mail.AccountConfig) ([]mail.FetchedMessage, error) {
	f.LastAccount = account
	if !strings.EqualFold(account.Folder, "INBOX") {
		return []mail.FetchedMessage{}, nil
	}
	return f.Messages, nil
}

func (f *capturingFetcher) ListInboxUIDs(account mail.AccountConfig) ([]string, error) {
	return f.ListFolderUIDs(account)
}

func (f *capturingFetcher) ListFolderUIDs(account mail.AccountConfig) ([]string, error) {
	f.LastAccount = account
	uids := make([]string, 0, len(f.Messages))
	for _, message := range f.Messages {
		uids = append(uids, message.UID)
	}
	return uids, nil
}

func (f *capturingFetcher) FetchInboxByUIDs(account mail.AccountConfig, uids []string) ([]mail.FetchedMessage, error) {
	return f.FetchFolderByUIDs(account, uids)
}

func (f *capturingFetcher) FetchFolderByUIDs(account mail.AccountConfig, uids []string) ([]mail.FetchedMessage, error) {
	f.LastAccount = account
	want := make(map[string]bool, len(uids))
	for _, uid := range uids {
		want[uid] = true
	}
	messages := make([]mail.FetchedMessage, 0, len(uids))
	for _, message := range f.Messages {
		if want[message.UID] {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func (f *capturingFetcher) DeleteInboxUIDs(account mail.AccountConfig, uids []string) error {
	return f.DeleteFolderUIDs(account, uids)
}

func (f *capturingFetcher) DeleteFolderUIDs(account mail.AccountConfig, uids []string) error {
	f.LastAccount = account
	return nil
}

type blockingFullSyncFetcher struct {
	*mail.FakeFetcher
	started chan struct{}
	release chan struct{}
}

func newBlockingFullSyncFetcher(count int) *blockingFullSyncFetcher {
	messages := make([]mail.FetchedMessage, 0, count)
	for i := 1; i <= count; i++ {
		messages = append(messages, mail.FetchedMessage{
			UID:        fmt.Sprint(i),
			MessageID:  fmt.Sprintf("<blocking-%03d@example.com>", i),
			Subject:    fmt.Sprintf("阻塞同步 %03d", i),
			From:       "archive@example.com",
			To:         []string{"first@example.com"},
			SentAt:     "2026-07-01T10:00:00+08:00",
			TextBody:   "阻塞同步正文",
			RawContent: fmt.Sprintf("Subject: 阻塞同步 %03d\r\n\r\n阻塞同步正文", i),
		})
	}
	return &blockingFullSyncFetcher{
		FakeFetcher: &mail.FakeFetcher{Messages: messages},
		started:     make(chan struct{}),
		release:     make(chan struct{}),
	}
}

func (f *blockingFullSyncFetcher) FetchInboxByUIDs(account mail.AccountConfig, uids []string) ([]mail.FetchedMessage, error) {
	return f.FetchFolderByUIDs(account, uids)
}

func (f *blockingFullSyncFetcher) FetchFolderByUIDs(account mail.AccountConfig, uids []string) ([]mail.FetchedMessage, error) {
	select {
	case <-f.started:
	default:
		close(f.started)
	}
	<-f.release
	return f.FakeFetcher.FetchFolderByUIDs(account, uids)
}

func performRequest(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func performMultipartRequest(handler http.Handler, method, path string, body []byte, contentType, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func registerTestUser(t *testing.T, router http.Handler, username, email string) string {
	t.Helper()

	body := `{"username":"` + username + `","email":"` + email + `","password":"password123"}`
	resp := performRequest(router, http.MethodPost, "/api/v1/auth/register", body, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("register %s failed: %d %s", username, resp.Code, resp.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}
	return nestedString(t, envelope, "data", "token")
}

func createTestAccount(t *testing.T, router http.Handler, token string) string {
	t.Helper()

	body := `{"displayName":"工作邮箱","email":"first@example.com","imapHost":"imap.example.com","imapPort":993,"imapTls":true,"imapUsername":"first@example.com","imapPassword":"mail-password","sentFolder":"Sent","pollIntervalMinutes":10,"enabled":true}`
	resp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts", body, token)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create account failed: %d %s", resp.Code, resp.Body.String())
	}
	return nestedString(t, decodeEnvelope(t, resp.Body.Bytes()), "data", "id")
}

func waitForFullSyncStatus(t *testing.T, router http.Handler, token, accountID, expected string) map[string]any {
	t.Helper()

	var data map[string]any
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp := performRequest(router, http.MethodGet, "/api/v1/mail-accounts/"+accountID+"/sync-status", "", token)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected sync status 200, got %d: %s", resp.Code, resp.Body.String())
		}
		data = decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)
		if data["fullSyncStatus"] == expected {
			return data
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("full sync status did not become %q, last status %#v", expected, data)
	return nil
}

func decodeEnvelope(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return envelope
}

func listItemCount(t *testing.T, body []byte) int {
	t.Helper()

	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", envelope["data"])
	}
	items, ok := data["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %#v", data["items"])
	}
	return len(items)
}

func nestedString(t *testing.T, input map[string]any, keys ...string) string {
	t.Helper()

	var current any = input
	for _, key := range keys {
		asMap, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected map while reading %v, got %#v", keys, current)
		}
		current = asMap[key]
	}

	value, ok := current.(string)
	if !ok {
		t.Fatalf("expected string at %v, got %#v", keys, current)
	}
	return value
}

func firstImageSource(t *testing.T, htmlBody string) string {
	t.Helper()
	marker := `src="`
	start := strings.Index(htmlBody, marker)
	if start < 0 {
		t.Fatalf("expected image src in html body, got %q", htmlBody)
	}
	start += len(marker)
	end := strings.Index(htmlBody[start:], `"`)
	if end < 0 {
		t.Fatalf("expected image src to be closed, got %q", htmlBody)
	}
	return htmlBody[start : start+end]
}

func nestedFloat64(t *testing.T, input map[string]any, keys ...string) float64 {
	t.Helper()

	var current any = input
	for _, key := range keys {
		asMap, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected map while reading %v, got %#v", keys, current)
		}
		current = asMap[key]
	}

	value, ok := current.(float64)
	if !ok {
		t.Fatalf("expected number at %v, got %#v", keys, current)
	}
	return value
}

func firstListItemID(t *testing.T, body []byte) string {
	t.Helper()
	item := firstListItem(t, body)
	id, ok := item["id"].(string)
	if !ok {
		t.Fatalf("expected string id, got %#v", item["id"])
	}
	return id
}

func firstListItem(t *testing.T, body []byte) map[string]any {
	t.Helper()

	envelope := decodeEnvelope(t, body)
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", envelope["data"])
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected non-empty items, got %#v", data["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first item object, got %#v", items[0])
	}
	return item
}

func listSubjects(t *testing.T, body []byte) []string {
	t.Helper()

	envelope := decodeEnvelope(t, body)
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", envelope["data"])
	}
	items, ok := data["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %#v", data["items"])
	}
	subjects := make([]string, 0, len(items))
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			t.Fatalf("expected item object, got %#v", rawItem)
		}
		subject, ok := item["subject"].(string)
		if !ok {
			t.Fatalf("expected subject string, got %#v", item["subject"])
		}
		subjects = append(subjects, subject)
	}
	return subjects
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
