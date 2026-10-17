package api

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/storage"
)

func TestCursorPagesSameDateIsolationAndNewMail(t *testing.T) {
	cfg := config.Default()
	cfg.App.DataDir = t.TempDir()
	cfg.Database.Path = filepath.Join(cfg.App.DataDir, "cursor.db")
	app, err := NewAppWithDependencies(cfg, &mail.FakeFetcher{}, &oauth.FakeMicrosoftExchanger{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	router := app.Routes()
	token := registerTestUser(t, router, "cursor", "cursor@example.com")
	other := registerTestUser(t, router, "cursor-other", "other@example.com")
	user, _ := app.store.FindUserByAccount("cursor")
	account, err := app.store.CreateMailAccount(storage.MailAccount{UserID: user.ID, Email: "cursor@example.com", DisplayName: "游标", IMAPHost: "example.com", IMAPPort: 993, IMAPUsername: "cursor", IMAPPasswordEncoded: "encrypted", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	at := sql.NullTime{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	var ids []int64
	for i := 0; i < 7; i++ {
		m, _, e := app.store.InsertMailMessageIfNew(storage.CreateMailMessageParams{UserID: user.ID, AccountID: account.ID, Folder: "INBOX", IMAPUID: fmt.Sprint(i), Subject: "中文 601", ReceivedAt: at})
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, m.ID)
	}
	base := "/api/v1/messages?includeTotal=false&pageSize=2&keyword=601"
	request := func(path, auth string) map[string]any {
		t.Helper()
		r := performRequest(router, "GET", path, "", auth)
		if r.Code != 200 {
			t.Fatalf("列表失败: %s", r.Body.String())
		}
		return decodeEnvelope(t, r.Body.Bytes())["data"].(map[string]any)
	}
	first := request(base, token)
	cursor := first["nextCursor"].(string)
	for _, tc := range []struct{ path, auth string }{
		{base + "&cursor=" + url.QueryEscape(cursor), other},
		{base + "&from=other&cursor=" + url.QueryEscape(cursor), token},
		{base + "&cursor=broken", token},
	} {
		r := performRequest(router, "GET", tc.path, "", tc.auth)
		if r.Code != 400 || decodeEnvelope(t, r.Body.Bytes())["httpCode"] != float64(400) {
			t.Fatalf("非法游标未拒绝: %s", r.Body.String())
		}
	}
	// 翻页时到达的新邮件只影响刷新后的第一页，不使续页重复。
	_, _, err = app.store.InsertMailMessageIfNew(storage.CreateMailMessageParams{UserID: user.ID, AccountID: account.ID, Folder: "INBOX", IMAPUID: "new", Subject: "601", ReceivedAt: sql.NullTime{Time: time.Now(), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	data := first
	for page := 1; ; page++ {
		for _, value := range data["items"].([]any) {
			id := value.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatal("重复邮件", id)
			}
			seen[id] = true
		}
		if !data["hasMore"].(bool) {
			break
		}
		if page > 6 {
			t.Fatal("游标未收敛")
		}
		data = request(base+"&page="+fmt.Sprint(page+1)+"&cursor="+url.QueryEscape(data["nextCursor"].(string)), token)
	}
	if len(seen) != len(ids) {
		t.Fatalf("漏信: got %d want %d", len(seen), len(ids))
	}
	count := request("/api/v1/messages/count?keyword=601&cursor="+url.QueryEscape(cursor), token)
	if count["total"] != float64(8) {
		t.Fatalf("游标影响总数: %+v", count)
	}
}
