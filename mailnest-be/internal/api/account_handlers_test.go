package api

import (
	"net/http"
	"testing"

	"mailnest-be/internal/mail"
)

func TestMailAccountsAreIsolatedByUser(t *testing.T) {
	router := newTestRouter(t, true)

	firstToken := registerTestUser(t, router, "first", "first@example.com")
	secondToken := registerTestUser(t, router, "second", "second@example.com")

	createBody := `{"displayName":"工作邮箱","email":"first@example.com","imapHost":"imap.example.com","imapPort":993,"imapTls":true,"imapUsername":"first@example.com","imapPassword":"mail-password","pollIntervalMinutes":10,"enabled":true}`
	createResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts", createBody, firstToken)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected create mail account status 201, got %d: %s", createResp.Code, createResp.Body.String())
	}

	firstList := performRequest(router, http.MethodGet, "/api/v1/mail-accounts", "", firstToken)
	if firstList.Code != http.StatusOK {
		t.Fatalf("expected first list status 200, got %d: %s", firstList.Code, firstList.Body.String())
	}
	if listItemCount(t, firstList.Body.Bytes()) != 1 {
		t.Fatalf("expected first user to see 1 account, got %s", firstList.Body.String())
	}

	secondList := performRequest(router, http.MethodGet, "/api/v1/mail-accounts", "", secondToken)
	if secondList.Code != http.StatusOK {
		t.Fatalf("expected second list status 200, got %d: %s", secondList.Code, secondList.Body.String())
	}
	if listItemCount(t, secondList.Body.Bytes()) != 0 {
		t.Fatalf("expected second user to see 0 accounts, got %s", secondList.Body.String())
	}
}

func TestUpdateMailAccountPreservesPasswordWhenEmpty(t *testing.T) {
	fetcher := &capturingFetcher{}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "update-account", "update-account@example.com")
	accountID := createTestAccount(t, router, token)

	updateBody := `{
		"displayName":"更新后的邮箱",
		"email":"updated@example.com",
		"imapHost":"imap.updated.example.com",
		"imapPort":143,
		"imapTls":false,
		"imapUsername":"updated@example.com",
		"imapPassword":"",
		"pollIntervalMinutes":30,
		"enabled":false
	}`
	updateResp := performRequest(router, http.MethodPut, "/api/v1/mail-accounts/"+accountID, updateBody, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update account status 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}
	data := decodeEnvelope(t, updateResp.Body.Bytes())["data"].(map[string]any)
	if data["displayName"] != "更新后的邮箱" || data["imapHost"] != "imap.updated.example.com" || data["enabled"] != false {
		t.Fatalf("expected updated account payload, got %#v", data)
	}

	testResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/test-connection", "", token)
	if testResp.Code != http.StatusOK {
		t.Fatalf("expected test connection status 200, got %d: %s", testResp.Code, testResp.Body.String())
	}
	if fetcher.LastAccount.Password != "mail-password" {
		t.Fatalf("expected original password to be preserved, got %q", fetcher.LastAccount.Password)
	}
	if fetcher.LastAccount.Host != "imap.updated.example.com" || fetcher.LastAccount.Port != 143 || fetcher.LastAccount.TLS {
		t.Fatalf("expected updated connection settings, got %#v", fetcher.LastAccount)
	}
}

func TestMailAccountFoldersEndpointListsSentCandidates(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Folders: []mail.FolderInfo{
			{Name: "INBOX", Attributes: []string{"\\HasNoChildren"}},
			{Name: "已发送邮件", Attributes: []string{"\\Sent"}},
			{Name: "Drafts", Attributes: []string{"\\Drafts"}},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "folder-user", "folder-user@example.com")
	accountID := createTestAccount(t, router, token)

	resp := performRequest(router, http.MethodGet, "/api/v1/mail-accounts/"+accountID+"/folders", "", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected folders status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	items := decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("expected 3 folders, got %#v", items)
	}
	sent := items[1].(map[string]any)
	if sent["name"] != "已发送邮件" || sent["sentCandidate"] != true {
		t.Fatalf("expected sent folder candidate, got %#v", sent)
	}
}
