package api

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/storage"
)

func TestDeferredListsAndContactResolver(t *testing.T) {
	cfg := config.Default()
	cfg.App.DataDir = t.TempDir()
	cfg.Database.Path = filepath.Join(cfg.App.DataDir, "mail.db")
	app, err := NewAppWithDependencies(cfg, &mail.FakeFetcher{}, &oauth.FakeMicrosoftExchanger{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	router := app.Routes()
	token := registerTestUser(t, router, "deferred", "deferred@example.com")
	other := registerTestUser(t, router, "other-deferred", "other-deferred@example.com")
	user, err := app.store.FindUserByAccount("deferred")
	if err != nil {
		t.Fatal(err)
	}
	account, err := app.store.CreateMailAccount(storage.MailAccount{UserID: user.ID, DisplayName: "延迟统计", Email: "mail@example.com", IMAPHost: "example.com", IMAPPort: 993, IMAPUsername: "mail", IMAPPasswordEncoded: "encrypted", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		thread, err := app.store.CreateMailThread(storage.CreateMailThreadParams{UserID: user.ID, AccountID: account.ID, Subject: "601"})
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = app.store.InsertMailMessageIfNew(storage.CreateMailMessageParams{UserID: user.ID, AccountID: account.ID, ThreadID: sql.NullInt64{Int64: thread.ID, Valid: true}, Folder: "INBOX", IMAPUID: fmt.Sprint(i), Subject: "601", FromAddr: "sender@example.com"})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"messages", "threads"} {
		for _, tc := range []struct {
			suffix, token string
			total         any
			items         int
			more          bool
		}{
			{"?keyword=601&pageSize=2", token, float64(3), 2, true},
			{"?keyword=601&pageSize=2&includeTotal=false", token, nil, 2, true},
			{"?keyword=601&pageSize=2&page=2&includeTotal=false", token, nil, 1, false},
			{"?keyword=不存在&includeTotal=false", token, nil, 0, false},
			{"?keyword=601&pageSize=2&includeTotal=false", other, nil, 0, false},
		} {
			resp := performRequest(router, "GET", "/api/v1/"+kind+tc.suffix, "", tc.token)
			envelope := decodeEnvelope(t, resp.Body.Bytes())
			data := envelope["data"].(map[string]any)
			if resp.Code != 200 || envelope["httpCode"] != float64(200) || data["total"] != tc.total || len(data["items"].([]any)) != tc.items || data["hasMore"] != tc.more {
				t.Fatalf("%s: %s", kind, resp.Body.String())
			}
		}
		resp := performRequest(router, "GET", "/api/v1/"+kind+"/count?keyword=601", "", token)
		data := decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)
		if resp.Code != 200 || data["total"] != float64(3) || data["revision"] == "" {
			t.Fatalf("统计失败 %s", resp.Body.String())
		}
		if resp := performRequest(router, "GET", "/api/v1/"+kind+"/count", "", ""); resp.Code != 401 {
			t.Fatal("统计接口未鉴权")
		}
	}
	if _, err = app.store.CreateContact(storage.CreateContactParams{UserID: user.ID, Email: "sender@example.com", Nickname: "发件人昵称"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		token       string
		count, code int
	}{{token, 1, 200}, {other, 0, 200}, {"", 0, 401}} {
		resp := performRequest(router, "POST", "/api/v1/contacts/resolve", `{"addresses":["名字 <sender@example.com>"]}`, tc.token)
		if resp.Code != tc.code {
			t.Fatalf("联系人解析状态 %s", resp.Body.String())
		}
		if tc.code == 200 {
			data := decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)
			if len(data["items"].([]any)) != tc.count {
				t.Fatal("联系人隔离错误")
			}
		}
	}
}
