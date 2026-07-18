package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
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

func TestAuthFlowRegisterLoginAndMe(t *testing.T) {
	router := newTestRouter(t, true)

	registerBody := `{"username":"demo","email":"demo@example.com","password":"password123"}`
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

	loginBody := `{"account":"demo","password":"password123"}`
	loginResp := performRequest(router, http.MethodPost, "/api/v1/auth/login", loginBody, "")
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

	oldLoginResp := performRequest(router, http.MethodPost, "/api/v1/auth/login", `{"account":"change-password","password":"password123"}`, "")
	if oldLoginResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password login status 401, got %d: %s", oldLoginResp.Code, oldLoginResp.Body.String())
	}

	newLoginResp := performRequest(router, http.MethodPost, "/api/v1/auth/login", `{"account":"change-password","password":"new-password-123"}`, "")
	if newLoginResp.Code != http.StatusOK {
		t.Fatalf("expected new password login status 200, got %d: %s", newLoginResp.Code, newLoginResp.Body.String())
	}
}

func TestProfileCanBeUpdatedAndAvatarCanBeUploaded(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "profile-user", "profile-user@example.com")

	updateResp := performRequest(router, http.MethodPut, "/api/v1/profile", `{"nickname":"信匣用户","bio":"用 Mail Nest 管理邮件"}`, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update profile status 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}
	data := decodeEnvelope(t, updateResp.Body.Bytes())["data"].(map[string]any)
	if data["nickname"] != "信匣用户" || data["bio"] != "用 Mail Nest 管理邮件" {
		t.Fatalf("expected updated profile data, got %#v", data)
	}

	meResp := performRequest(router, http.MethodGet, "/api/v1/auth/me", "", token)
	if meResp.Code != http.StatusOK {
		t.Fatalf("expected me status 200, got %d: %s", meResp.Code, meResp.Body.String())
	}
	if nestedString(t, decodeEnvelope(t, meResp.Body.Bytes()), "data", "nickname") != "信匣用户" {
		t.Fatalf("expected me payload to include nickname, got %s", meResp.Body.String())
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

func TestContactsCanBeMaintainedAndAreIsolatedByUser(t *testing.T) {
	router := newTestRouter(t, true)

	firstToken := registerTestUser(t, router, "contact-first", "contact-first@example.com")
	secondToken := registerTestUser(t, router, "contact-second", "contact-second@example.com")

	createResp := performRequest(router, http.MethodPost, "/api/v1/contacts", `{
		"email":"Alice <alice@example.com>",
		"displayName":"",
		"nickname":"Alice",
		"phone":"123456",
		"company":"Example Inc.",
		"notes":"重要客户"
	}`, firstToken)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected create contact status 201, got %d: %s", createResp.Code, createResp.Body.String())
	}
	contactID := nestedString(t, decodeEnvelope(t, createResp.Body.Bytes()), "data", "id")
	if nestedString(t, decodeEnvelope(t, createResp.Body.Bytes()), "data", "email") != "alice@example.com" {
		t.Fatalf("expected normalized email, got %s", createResp.Body.String())
	}

	firstList := performRequest(router, http.MethodGet, "/api/v1/contacts?keyword=Alice", "", firstToken)
	if firstList.Code != http.StatusOK {
		t.Fatalf("expected first contact list status 200, got %d: %s", firstList.Code, firstList.Body.String())
	}
	if listItemCount(t, firstList.Body.Bytes()) != 1 {
		t.Fatalf("expected first user to see one contact, got %s", firstList.Body.String())
	}

	secondList := performRequest(router, http.MethodGet, "/api/v1/contacts", "", secondToken)
	if secondList.Code != http.StatusOK {
		t.Fatalf("expected second contact list status 200, got %d: %s", secondList.Code, secondList.Body.String())
	}
	if listItemCount(t, secondList.Body.Bytes()) != 0 {
		t.Fatalf("expected second user to see no contacts, got %s", secondList.Body.String())
	}

	updateResp := performRequest(router, http.MethodPut, "/api/v1/contacts/"+contactID, `{
		"email":"alice@example.com",
		"displayName":"Alice Zhang",
		"nickname":"阿丽",
		"phone":"654321",
		"company":"Mail Nest",
		"notes":"已更新"
	}`, firstToken)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update contact status 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}
	if nestedString(t, decodeEnvelope(t, updateResp.Body.Bytes()), "data", "name") != "阿丽" {
		t.Fatalf("expected preferred nickname in response, got %s", updateResp.Body.String())
	}

	deleteResp := performRequest(router, http.MethodDelete, "/api/v1/contacts/"+contactID, "", firstToken)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete contact status 200, got %d: %s", deleteResp.Code, deleteResp.Body.String())
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

func TestSyncMessagesAndMessageAccessAreIsolatedByUser(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "100",
				MessageID:  "<message-100@example.com>",
				Subject:    "第一封测试邮件",
				From:       "sender@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-06T12:00:00+08:00",
				TextBody:   "这是一封用于测试收取的邮件",
				HTMLBody:   "<p>这是一封用于测试收取的邮件</p>",
				RawContent: "Subject: 第一封测试邮件\r\n\r\n这是一封用于测试收取的邮件",
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)

	firstToken := registerTestUser(t, router, "first", "first@example.com")
	secondToken := registerTestUser(t, router, "second", "second@example.com")
	accountID := createTestAccount(t, router, firstToken)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", firstToken)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	if nestedFloat64(t, decodeEnvelope(t, syncResp.Body.Bytes()), "data", "newMessageCount") != 1 {
		t.Fatalf("expected first sync to add 1 message, got %s", syncResp.Body.String())
	}

	secondSyncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", firstToken)
	if secondSyncResp.Code != http.StatusOK {
		t.Fatalf("expected second sync status 200, got %d: %s", secondSyncResp.Code, secondSyncResp.Body.String())
	}
	if nestedFloat64(t, decodeEnvelope(t, secondSyncResp.Body.Bytes()), "data", "newMessageCount") != 0 {
		t.Fatalf("expected duplicate sync to add 0 messages, got %s", secondSyncResp.Body.String())
	}

	firstMessages := performRequest(router, http.MethodGet, "/api/v1/messages", "", firstToken)
	if firstMessages.Code != http.StatusOK {
		t.Fatalf("expected messages status 200, got %d: %s", firstMessages.Code, firstMessages.Body.String())
	}
	if listItemCount(t, firstMessages.Body.Bytes()) != 1 {
		t.Fatalf("expected first user to see 1 message, got %s", firstMessages.Body.String())
	}
	messageID := firstListItemID(t, firstMessages.Body.Bytes())

	detailResp := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID, "", firstToken)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected message detail status 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	if nestedString(t, decodeEnvelope(t, detailResp.Body.Bytes()), "data", "subject") != "第一封测试邮件" {
		t.Fatalf("expected detail subject, got %s", detailResp.Body.String())
	}

	secondMessages := performRequest(router, http.MethodGet, "/api/v1/messages", "", secondToken)
	if secondMessages.Code != http.StatusOK {
		t.Fatalf("expected second messages status 200, got %d: %s", secondMessages.Code, secondMessages.Body.String())
	}
	if listItemCount(t, secondMessages.Body.Bytes()) != 0 {
		t.Fatalf("expected second user to see 0 messages, got %s", secondMessages.Body.String())
	}

	forbiddenDetail := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID, "", secondToken)
	if forbiddenDetail.Code != http.StatusNotFound {
		t.Fatalf("expected second user detail status 404, got %d: %s", forbiddenDetail.Code, forbiddenDetail.Body.String())
	}
}

func TestSyncIncludesSentFolderAndFiltersSentMessages(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		FolderMessages: map[string][]mail.FetchedMessage{
			"INBOX": {
				{
					UID:        "inbox-1",
					MessageID:  "<inbox-1@example.com>",
					Subject:    "收到的邮件",
					From:       "sender@example.com",
					To:         []string{"first@example.com"},
					SentAt:     "2026-07-06T12:00:00+08:00",
					TextBody:   "收件箱正文",
					RawContent: "Subject: 收到的邮件\r\n\r\n收件箱正文",
				},
			},
			"Sent Messages": {
				{
					UID:        "sent-1",
					MessageID:  "<sent-1@example.com>",
					Subject:    "已发送邮件",
					From:       "first@example.com",
					To:         []string{"receiver@example.com"},
					SentAt:     "2026-07-06T13:00:00+08:00",
					TextBody:   "发件箱正文",
					RawContent: "Subject: 已发送邮件\r\n\r\n发件箱正文",
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "sent-user", "sent-user@example.com")

	createBody := `{"displayName":"工作邮箱","email":"first@example.com","imapHost":"imap.example.com","imapPort":993,"imapTls":true,"imapUsername":"first@example.com","imapPassword":"mail-password","sentFolder":"Sent Messages","pollIntervalMinutes":10,"enabled":true}`
	createResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts", createBody, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected create account status 201, got %d: %s", createResp.Code, createResp.Body.String())
	}
	accountData := decodeEnvelope(t, createResp.Body.Bytes())["data"].(map[string]any)
	if accountData["sentFolder"] != "Sent Messages" {
		t.Fatalf("expected sent folder in account payload, got %#v", accountData)
	}
	accountID := accountData["id"].(string)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	if nestedFloat64(t, decodeEnvelope(t, syncResp.Body.Bytes()), "data", "newMessageCount") != 2 {
		t.Fatalf("expected sync to add inbox and sent messages, got %s", syncResp.Body.String())
	}

	inboxResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=inbox", "", token)
	if got := listSubjects(t, inboxResp.Body.Bytes()); len(got) != 1 || got[0] != "收到的邮件" {
		t.Fatalf("expected only inbox message, got %#v", got)
	}
	sentResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=sent", "", token)
	if got := listSubjects(t, sentResp.Body.Bytes()); len(got) != 1 || got[0] != "已发送邮件" {
		t.Fatalf("expected only sent message, got %#v", got)
	}
	allResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=all", "", token)
	if got := listSubjects(t, allResp.Body.Bytes()); len(got) != 2 {
		t.Fatalf("expected all messages to include inbox and sent, got %#v", got)
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

func TestSyncSkipsMissingSentFolderWithoutDroppingInboxMessages(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		FolderMessages: map[string][]mail.FetchedMessage{
			"INBOX": {
				{
					UID:        "inbox-1",
					MessageID:  "<missing-sent-inbox-1@example.com>",
					Subject:    "收件箱仍可同步",
					From:       "sender@example.com",
					To:         []string{"first@example.com"},
					SentAt:     "2026-07-06T12:00:00+08:00",
					TextBody:   "发件箱目录不存在时，收件箱仍应入库",
					RawContent: "Subject: 收件箱仍可同步\r\n\r\n正文",
				},
			},
		},
		FolderErrors: map[string]error{
			"Sent": mail.ErrFolderNotFound,
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "missing-sent-user", "missing-sent@example.com")
	accountID := createTestAccount(t, router, token)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	data := decodeEnvelope(t, syncResp.Body.Bytes())["data"].(map[string]any)
	if data["newMessageCount"] != float64(1) {
		t.Fatalf("expected one inbox message, got %#v", data)
	}
	warnings := data["warnings"].([]any)
	if len(warnings) != 1 || !strings.Contains(warnings[0].(string), "Sent") {
		t.Fatalf("expected sent folder warning, got %#v", data["warnings"])
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=inbox", "", token)
	if got := listSubjects(t, listResp.Body.Bytes()); len(got) != 1 || got[0] != "收件箱仍可同步" {
		t.Fatalf("expected inbox message to be saved, got %#v", got)
	}
}

func TestMessageDetailReturnsAttachmentsAndInlineCIDImages(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "with-attachments",
				MessageID:  "<with-attachments@example.com>",
				Subject:    "带附件邮件",
				From:       "sender@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-06T12:00:00+08:00",
				TextBody:   "请查看图片和附件",
				HTMLBody:   `<p>请查看图片</p><img src="cid:inline-image-1">`,
				RawContent: "Subject: 带附件邮件\r\n\r\n请查看图片和附件",
				Attachments: []mail.FetchedAttachment{
					{
						Filename:    "inline.png",
						ContentType: "image/png",
						ContentID:   "inline-image-1",
						Inline:      true,
						Data:        []byte("inline-image-bytes"),
					},
					{
						Filename:    "report.pdf",
						ContentType: "application/pdf",
						Data:        []byte("%PDF-1.4"),
					},
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "attachment-user", "attachment-user@example.com")
	accountID := createTestAccount(t, router, token)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected messages status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	first := firstListItem(t, listResp.Body.Bytes())
	if first["hasAttachments"] != true {
		t.Fatalf("expected list item to have attachments, got %#v", first)
	}
	messageID, ok := first["id"].(string)
	if !ok {
		t.Fatalf("expected string id, got %#v", first["id"])
	}

	detailResp := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID, "", token)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	data := decodeEnvelope(t, detailResp.Body.Bytes())["data"].(map[string]any)
	htmlBody, ok := data["htmlBody"].(string)
	if !ok || !strings.Contains(htmlBody, `/inline-content?`) || strings.Contains(strings.ToLower(htmlBody), "cid:") {
		t.Fatalf("expected cid image to be rewritten as signed inline URL, got %#v", data["htmlBody"])
	}
	inlineURL := firstImageSource(t, htmlBody)
	inlineResp := performRequest(router, http.MethodGet, inlineURL, "", "")
	if inlineResp.Code != http.StatusOK {
		t.Fatalf("expected inline image status 200, got %d: %s", inlineResp.Code, inlineResp.Body.String())
	}
	if inlineResp.Body.String() != "inline-image-bytes" {
		t.Fatalf("expected inline image bytes, got %q", inlineResp.Body.String())
	}
	sigIndex := strings.Index(inlineURL, "sig=")
	if sigIndex < 0 {
		t.Fatalf("expected signed inline url, got %q", inlineURL)
	}
	tamperedInlineResp := performRequest(router, http.MethodGet, inlineURL[:sigIndex]+"sig=bad", "", "")
	if tamperedInlineResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected tampered inline image status 401, got %d: %s", tamperedInlineResp.Code, tamperedInlineResp.Body.String())
	}
	attachments, ok := data["attachments"].([]any)
	if !ok || len(attachments) != 2 {
		t.Fatalf("expected two attachments, got %#v", data["attachments"])
	}
	normalAttachment := attachments[1].(map[string]any)
	if normalAttachment["filename"] != "report.pdf" || normalAttachment["inline"] != false {
		t.Fatalf("expected normal attachment metadata, got %#v", normalAttachment)
	}
	downloadURL, ok := normalAttachment["downloadUrl"].(string)
	if !ok || downloadURL == "" {
		t.Fatalf("expected attachment downloadUrl, got %#v", normalAttachment)
	}

	downloadResp := performRequest(router, http.MethodGet, downloadURL, "", token)
	if downloadResp.Code != http.StatusOK {
		t.Fatalf("expected attachment download status 200, got %d: %s", downloadResp.Code, downloadResp.Body.String())
	}
	if downloadResp.Body.String() != "%PDF-1.4" {
		t.Fatalf("expected attachment content, got %q", downloadResp.Body.String())
	}
}

func TestMessageDetailTreatsCIDReferencedAttachmentAsInline(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "foxmail-inline-false",
				MessageID:  "<foxmail-inline-false@example.com>",
				Subject:    "Foxmail 内嵌图片",
				From:       "sender@example.com",
				To:         []string{"receiver@example.com"},
				HTMLBody:   `<p>正文图片</p><img src="CID:%3C_Foxmail.1@55d24ed9-1d1b-3c80-0b94-c95e7a27a898%3E">`,
				RawContent: "Subject: Foxmail 内嵌图片\r\n\r\n正文图片",
				Attachments: []mail.FetchedAttachment{
					{
						Filename:    "InsertPic_1F7E.jpg",
						ContentType: "image/jpeg",
						ContentID:   "_Foxmail.1@55d24ed9-1d1b-3c80-0b94-c95e7a27a898",
						Inline:      false,
						Data:        []byte("image-bytes"),
					},
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "foxmail-user", "foxmail-user@example.com")
	accountID := createTestAccount(t, router, token)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected messages status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	messageID := firstListItem(t, listResp.Body.Bytes())["id"].(string)

	detailResp := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID, "", token)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	data := decodeEnvelope(t, detailResp.Body.Bytes())["data"].(map[string]any)
	htmlBody := data["htmlBody"].(string)
	if !strings.Contains(htmlBody, `/inline-content?`) || strings.Contains(strings.ToLower(htmlBody), "cid:") {
		t.Fatalf("expected cid referenced attachment to be rewritten, got %q", htmlBody)
	}
	attachments := data["attachments"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("expected one attachment, got %#v", data["attachments"])
	}
	inlineAttachment := attachments[0].(map[string]any)
	if inlineAttachment["filename"] != "InsertPic_1F7E.jpg" || inlineAttachment["inline"] != true {
		t.Fatalf("expected cid referenced attachment to be marked inline, got %#v", inlineAttachment)
	}
}

func TestMessageDetailReplacesMissingCIDImageWithPlaceholder(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "missing-cid-image",
				MessageID:  "<missing-cid-image@example.com>",
				Subject:    "缺失内嵌图片",
				From:       "sender@example.com",
				To:         []string{"receiver@example.com"},
				HTMLBody:   `<p>正文图片缺失</p><img src="cid:missing-inline-image">`,
				RawContent: "Subject: 缺失内嵌图片\r\n\r\n正文图片缺失",
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "missing-cid-user", "missing-cid-user@example.com")
	accountID := createTestAccount(t, router, token)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected messages status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	messageID := firstListItem(t, listResp.Body.Bytes())["id"].(string)

	detailResp := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID, "", token)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	htmlBody := nestedString(t, decodeEnvelope(t, detailResp.Body.Bytes()), "data", "htmlBody")
	if strings.Contains(strings.ToLower(htmlBody), "cid:") {
		t.Fatalf("expected missing cid image to be replaced, got %q", htmlBody)
	}
	if !strings.Contains(htmlBody, "data:image/svg+xml") {
		t.Fatalf("expected missing cid image placeholder, got %q", htmlBody)
	}
}

func TestMessageDetailConvertsInlineTIFFToPNG(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "inline-tiff",
				MessageID:  "<inline-tiff@example.com>",
				Subject:    "内嵌 TIFF 图片",
				From:       "sender@example.com",
				To:         []string{"receiver@example.com"},
				HTMLBody:   `<p>截图如下</p><img src="cid:inline-tiff-1" alt="粘贴的图形-1.tiff">`,
				RawContent: "Subject: 内嵌 TIFF 图片\r\n\r\n截图如下",
				Attachments: []mail.FetchedAttachment{
					{
						Filename:    "pasted-image.tiff",
						ContentType: "image/tiff",
						ContentID:   "inline-tiff-1",
						Inline:      true,
						Data:        tinyTIFFImage(),
					},
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "tiff-user", "tiff-user@example.com")
	accountID := createTestAccount(t, router, token)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected messages status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	messageID := firstListItem(t, listResp.Body.Bytes())["id"].(string)

	detailResp := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID, "", token)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	htmlBody := nestedString(t, decodeEnvelope(t, detailResp.Body.Bytes()), "data", "htmlBody")
	if !strings.Contains(htmlBody, `src="data:image/png;base64,`) {
		t.Fatalf("expected inline tiff image to be converted to png data URL, got %q", htmlBody)
	}
	if strings.Contains(htmlBody, "cid:inline-tiff-1") || strings.Contains(htmlBody, "data:image/tiff") {
		t.Fatalf("expected no cid or tiff data URL in html body, got %q", htmlBody)
	}
}

func tinyTIFFImage() []byte {
	return []byte{
		0x49, 0x49, 0x2a, 0x00, 0x08, 0x00, 0x00, 0x00,
		0x0a, 0x00,
		0x00, 0x01, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x01, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x02, 0x01, 0x03, 0x00, 0x03, 0x00, 0x00, 0x00, 0x86, 0x00, 0x00, 0x00,
		0x03, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x06, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x11, 0x01, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x8c, 0x00, 0x00, 0x00,
		0x15, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00,
		0x16, 0x01, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x17, 0x01, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00,
		0x1c, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x08, 0x00, 0x08, 0x00, 0x08, 0x00,
		0xff, 0x00, 0x00,
	}
}

func TestDuplicateSyncBackfillsMissingAttachments(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "legacy-message",
				MessageID:  "<legacy-message@example.com>",
				Subject:    "历史邮件",
				From:       "sender@example.com",
				To:         []string{"first@example.com"},
				HTMLBody:   `<p>历史邮件</p><img src="cid:inline-image-1">`,
				RawContent: "Subject: 历史邮件\r\n\r\n历史邮件",
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "legacy-user", "legacy-user@example.com")
	accountID := createTestAccount(t, router, token)

	firstSync := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if firstSync.Code != http.StatusOK {
		t.Fatalf("expected first sync status 200, got %d: %s", firstSync.Code, firstSync.Body.String())
	}
	if nestedFloat64(t, decodeEnvelope(t, firstSync.Body.Bytes()), "data", "newMessageCount") != 1 {
		t.Fatalf("expected first sync to add message, got %s", firstSync.Body.String())
	}

	fetcher.Messages[0].Attachments = []mail.FetchedAttachment{
		{
			Filename:    "inline.png",
			ContentType: "image/png",
			ContentID:   "inline-image-1",
			Inline:      true,
			Data:        []byte("inline-image-bytes"),
		},
	}
	secondSync := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if secondSync.Code != http.StatusOK {
		t.Fatalf("expected second sync status 200, got %d: %s", secondSync.Code, secondSync.Body.String())
	}
	if nestedFloat64(t, decodeEnvelope(t, secondSync.Body.Bytes()), "data", "newMessageCount") != 0 {
		t.Fatalf("expected duplicate sync to add 0 messages, got %s", secondSync.Body.String())
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected messages status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	first := firstListItem(t, listResp.Body.Bytes())
	if first["hasAttachments"] != true {
		t.Fatalf("expected backfilled message to have attachments, got %#v", first)
	}

	messageID := first["id"].(string)
	detailResp := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID, "", token)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	htmlBody := nestedString(t, decodeEnvelope(t, detailResp.Body.Bytes()), "data", "htmlBody")
	if !strings.Contains(htmlBody, `/inline-content?`) || strings.Contains(strings.ToLower(htmlBody), "cid:") {
		t.Fatalf("expected backfilled cid image to be rewritten as signed inline URL, got %q", htmlBody)
	}
}

func TestFullSyncFetchesAllInboxMessagesAndReportsProgress(t *testing.T) {
	messages := make([]mail.FetchedMessage, 0, 75)
	for i := 1; i <= 75; i++ {
		messages = append(messages, mail.FetchedMessage{
			UID:        fmt.Sprint(i),
			MessageID:  fmt.Sprintf("<full-%03d@example.com>", i),
			Subject:    fmt.Sprintf("历史邮件 %03d", i),
			From:       "archive@example.com",
			To:         []string{"first@example.com"},
			SentAt:     "2026-07-01T10:00:00+08:00",
			TextBody:   "历史邮件正文",
			RawContent: fmt.Sprintf("Subject: 历史邮件 %03d\r\n\r\n历史邮件正文", i),
		})
	}
	fetcher := &mail.FakeFetcher{Messages: messages}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "full-sync-user", "full-sync-user@example.com")
	accountID := createTestAccount(t, router, token)

	startResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/full-sync/start", "", token)
	if startResp.Code != http.StatusAccepted {
		t.Fatalf("expected full sync start status 202, got %d: %s", startResp.Code, startResp.Body.String())
	}

	status := waitForFullSyncStatus(t, router, token, accountID, "success")
	if status["fullSyncTotal"] != float64(75) || status["fullSyncProcessed"] != float64(75) || status["fullSyncNewCount"] != float64(75) {
		t.Fatalf("expected full sync progress 75/75, got %#v", status)
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/messages?pageSize=100", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected messages status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	if listItemCount(t, listResp.Body.Bytes()) != 75 {
		t.Fatalf("expected full sync to save all 75 messages, got %s", listResp.Body.String())
	}
}

func TestFullSyncCleanupDeletesOnlySyncedOldServerMessagesWhenEnabled(t *testing.T) {
	fetcher := &mail.FakeFetcher{Messages: []mail.FetchedMessage{
		{
			UID:        "1001",
			MessageID:  "<old@example.com>",
			Subject:    "旧邮件",
			From:       "archive@example.com",
			To:         []string{"first@example.com"},
			SentAt:     "2026-05-01T10:00:00+08:00",
			TextBody:   "旧邮件正文",
			RawContent: "Subject: 旧邮件\r\n\r\n旧邮件正文",
		},
		{
			UID:        "1002",
			MessageID:  "<new@example.com>",
			Subject:    "新邮件",
			From:       "archive@example.com",
			To:         []string{"first@example.com"},
			SentAt:     time.Now().Format(time.RFC3339),
			TextBody:   "新邮件正文",
			RawContent: "Subject: 新邮件\r\n\r\n新邮件正文",
		},
	}}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "cleanup-user", "cleanup-user@example.com")

	createBody := `{"displayName":"清理邮箱","email":"cleanup@example.com","imapHost":"imap.example.com","imapPort":993,"imapTls":true,"imapUsername":"cleanup@example.com","imapPassword":"mail-password","pollIntervalMinutes":10,"enabled":true,"cleanupEnabled":true,"cleanupRetentionDays":30}`
	createResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts", createBody, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create account failed: %d %s", createResp.Code, createResp.Body.String())
	}
	accountID := nestedString(t, decodeEnvelope(t, createResp.Body.Bytes()), "data", "id")

	startResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/full-sync/start", "", token)
	if startResp.Code != http.StatusAccepted {
		t.Fatalf("expected full sync start status 202, got %d: %s", startResp.Code, startResp.Body.String())
	}
	waitForFullSyncStatus(t, router, token, accountID, "success")

	if got := fmt.Sprint(fetcher.DeletedUIDs); got != "[1001]" {
		t.Fatalf("expected only old synced UID 1001 to be deleted from server, got %s", got)
	}
}

func TestFullSyncCanBeStopped(t *testing.T) {
	fetcher := newBlockingFullSyncFetcher(120)
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "stop-sync-user", "stop-sync-user@example.com")
	accountID := createTestAccount(t, router, token)

	startResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/full-sync/start", "", token)
	if startResp.Code != http.StatusAccepted {
		t.Fatalf("expected full sync start status 202, got %d: %s", startResp.Code, startResp.Body.String())
	}
	select {
	case <-fetcher.started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected full sync fetch to start")
	}

	stopResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/full-sync/stop", "", token)
	if stopResp.Code != http.StatusOK {
		t.Fatalf("expected full sync stop status 200, got %d: %s", stopResp.Code, stopResp.Body.String())
	}
	if nestedString(t, decodeEnvelope(t, stopResp.Body.Bytes()), "data", "fullSyncStatus") != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", stopResp.Body.String())
	}

	close(fetcher.release)
	status := waitForFullSyncStatus(t, router, token, accountID, "cancelled")
	if status["fullSyncError"] == nil {
		t.Fatalf("expected cancelled status to include message, got %#v", status)
	}
}

func TestListMessagesSupportsSearchFiltersAndUserIsolation(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "security-1",
				MessageID:  "<security-1@example.com>",
				Subject:    "网络安全整改通知",
				From:       "security@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-01T10:00:00+08:00",
				TextBody:   "请安装主机探针并反馈整改结果",
				HTMLBody:   "<p>请安装主机探针并反馈整改结果</p>",
				RawContent: "Subject: 网络安全整改通知\r\n\r\n请安装主机探针并反馈整改结果",
				Attachments: []mail.FetchedAttachment{
					{Filename: "hosts.xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Data: []byte("xlsx")},
				},
			},
			{
				UID:        "exam-1",
				MessageID:  "<exam-1@example.com>",
				Subject:    "认证考试倒计时",
				From:       "training@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-03T10:00:00+08:00",
				TextBody:   "实施服务能力认证考试还有五天",
				HTMLBody:   "<p>实施服务能力认证考试还有五天</p>",
				RawContent: "Subject: 认证考试倒计时\r\n\r\n实施服务能力认证考试还有五天",
			},
			{
				UID:        "system-1",
				MessageID:  "<system-1@example.com>",
				Subject:    "Container Manager 通知",
				From:       "notify@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-05T10:00:00+08:00",
				TextBody:   "postgres 容器意外停止",
				HTMLBody:   "<p>postgres 容器意外停止</p>",
				RawContent: "Subject: Container Manager 通知\r\n\r\npostgres 容器意外停止",
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	firstToken := registerTestUser(t, router, "search-first", "search-first@example.com")
	secondToken := registerTestUser(t, router, "search-second", "search-second@example.com")
	accountID := createTestAccount(t, router, firstToken)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", firstToken)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	cases := []struct {
		name     string
		query    string
		subjects []string
	}{
		{name: "keyword matches subject", query: "?keyword=网络安全", subjects: []string{"网络安全整改通知"}},
		{name: "keyword matches body", query: "?keyword=主机探针", subjects: []string{"网络安全整改通知"}},
		{name: "body filter", query: "?body=主机探针", subjects: []string{"网络安全整改通知"}},
		{name: "body filter does not match subject", query: "?body=Container", subjects: []string{}},
		{name: "from filter", query: "?from=training@example.com", subjects: []string{"认证考试倒计时"}},
		{name: "subject filter", query: "?subject=Container", subjects: []string{"Container Manager 通知"}},
		{name: "date range filter", query: "?dateFrom=2026-07-02&dateTo=2026-07-04", subjects: []string{"认证考试倒计时"}},
		{name: "attachment filter", query: "?hasAttachments=true", subjects: []string{"网络安全整改通知"}},
		{name: "account filter", query: "?accountId=" + accountID, subjects: []string{"Container Manager 通知", "认证考试倒计时", "网络安全整改通知"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performRequest(router, http.MethodGet, "/api/v1/messages"+tc.query, "", firstToken)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected messages status 200, got %d: %s", resp.Code, resp.Body.String())
			}
			if got := listSubjects(t, resp.Body.Bytes()); !equalStringSlices(got, tc.subjects) {
				t.Fatalf("expected subjects %#v, got %#v", tc.subjects, got)
			}
		})
	}

	secondResp := performRequest(router, http.MethodGet, "/api/v1/messages?keyword=网络安全", "", secondToken)
	if secondResp.Code != http.StatusOK {
		t.Fatalf("expected second user search status 200, got %d: %s", secondResp.Code, secondResp.Body.String())
	}
	if got := listSubjects(t, secondResp.Body.Bytes()); len(got) != 0 {
		t.Fatalf("expected second user to see no messages, got %#v", got)
	}
}

func TestMailFoldersCreateFilterAndDeleteWithoutRemovingMessages(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "folder-message",
				MessageID:  "<folder-message@example.com>",
				Subject:    "网络安全整改通知",
				From:       "security@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-01T10:00:00+08:00",
				TextBody:   "请安装主机探针",
				RawContent: "Subject: 网络安全整改通知\r\n\r\n请安装主机探针",
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	firstToken := registerTestUser(t, router, "folder-first", "folder-first@example.com")
	secondToken := registerTestUser(t, router, "folder-second", "folder-second@example.com")
	accountID := createTestAccount(t, router, firstToken)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", firstToken)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	messageID := firstListItemID(t, performRequest(router, http.MethodGet, "/api/v1/messages", "", firstToken).Body.Bytes())

	createResp := performRequest(router, http.MethodPost, "/api/v1/mail-folders", `{"name":"安全通知","color":"#1f66d1","sortOrder":10}`, firstToken)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected create folder status 201, got %d: %s", createResp.Code, createResp.Body.String())
	}
	folderID := nestedString(t, decodeEnvelope(t, createResp.Body.Bytes()), "data", "id")

	assignResp := performRequest(router, http.MethodPost, "/api/v1/messages/"+messageID+"/folder", `{"folderId":"`+folderID+`"}`, firstToken)
	if assignResp.Code != http.StatusOK {
		t.Fatalf("expected assign folder status 200, got %d: %s", assignResp.Code, assignResp.Body.String())
	}

	filterResp := performRequest(router, http.MethodGet, "/api/v1/messages?folderId="+folderID, "", firstToken)
	if filterResp.Code != http.StatusOK {
		t.Fatalf("expected folder filter status 200, got %d: %s", filterResp.Code, filterResp.Body.String())
	}
	if got := listSubjects(t, filterResp.Body.Bytes()); !equalStringSlices(got, []string{"网络安全整改通知"}) {
		t.Fatalf("expected folder message, got %#v", got)
	}

	secondFilterResp := performRequest(router, http.MethodGet, "/api/v1/messages?folderId="+folderID, "", secondToken)
	if secondFilterResp.Code != http.StatusOK {
		t.Fatalf("expected second folder filter status 200, got %d: %s", secondFilterResp.Code, secondFilterResp.Body.String())
	}
	if got := listSubjects(t, secondFilterResp.Body.Bytes()); len(got) != 0 {
		t.Fatalf("expected second user to see no folder messages, got %#v", got)
	}

	deleteResp := performRequest(router, http.MethodDelete, "/api/v1/mail-folders/"+folderID, "", firstToken)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete folder status 200, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
	allResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", firstToken)
	if got := listSubjects(t, allResp.Body.Bytes()); !equalStringSlices(got, []string{"网络安全整改通知"}) {
		t.Fatalf("expected deleting folder to keep message, got %#v", got)
	}
	emptyFolderResp := performRequest(router, http.MethodGet, "/api/v1/messages?folderId="+folderID, "", firstToken)
	if got := listSubjects(t, emptyFolderResp.Body.Bytes()); len(got) != 0 {
		t.Fatalf("expected deleted folder filter to be empty, got %#v", got)
	}
}

func TestMailRulesArchiveNewAndExistingMessages(t *testing.T) {
	fetcher := &mail.FakeFetcher{}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "rule-user", "rule-user@example.com")
	accountID := createTestAccount(t, router, token)

	folderResp := performRequest(router, http.MethodPost, "/api/v1/mail-folders", `{"name":"安全通知","color":"#1f66d1","sortOrder":10}`, token)
	if folderResp.Code != http.StatusCreated {
		t.Fatalf("expected folder status 201, got %d: %s", folderResp.Code, folderResp.Body.String())
	}
	folderID := nestedString(t, decodeEnvelope(t, folderResp.Body.Bytes()), "data", "id")

	ruleBody := `{
		"name":"安全通知归档",
		"enabled":true,
		"matchMode":"all",
		"targetFolderId":"` + folderID + `",
		"sortOrder":10,
		"conditions":[
			{"field":"subject","operator":"contains","value":"网络安全"},
			{"field":"has_attachments","operator":"is_true","value":""}
		]
	}`
	ruleResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules", ruleBody, token)
	if ruleResp.Code != http.StatusCreated {
		t.Fatalf("expected rule status 201, got %d: %s", ruleResp.Code, ruleResp.Body.String())
	}

	fetcher.Messages = []mail.FetchedMessage{
		{
			UID:        "new-rule-message",
			MessageID:  "<new-rule-message@example.com>",
			Subject:    "集团网络安全整改通知",
			From:       "security@example.com",
			To:         []string{"rule@example.com"},
			SentAt:     "2026-07-06T10:00:00+08:00",
			TextBody:   "请安装主机探针",
			RawContent: "Subject: 集团网络安全整改通知\r\n\r\n请安装主机探针",
			Attachments: []mail.FetchedAttachment{
				{Filename: "hosts.xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Data: []byte("xlsx")},
			},
		},
	}
	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	filterResp := performRequest(router, http.MethodGet, "/api/v1/messages?folderId="+folderID, "", token)
	if got := listSubjects(t, filterResp.Body.Bytes()); !equalStringSlices(got, []string{"集团网络安全整改通知"}) {
		t.Fatalf("expected new rule message in folder, got %#v", got)
	}

	otherFolderResp := performRequest(router, http.MethodPost, "/api/v1/mail-folders", `{"name":"培训通知","color":"#3b7a57","sortOrder":20}`, token)
	if otherFolderResp.Code != http.StatusCreated {
		t.Fatalf("expected other folder status 201, got %d: %s", otherFolderResp.Code, otherFolderResp.Body.String())
	}
	otherFolderID := nestedString(t, decodeEnvelope(t, otherFolderResp.Body.Bytes()), "data", "id")

	fetcher.Messages = []mail.FetchedMessage{
		{
			UID:        "old-rule-message",
			MessageID:  "<old-rule-message@example.com>",
			Subject:    "认证考试倒计时",
			From:       "training@example.com",
			To:         []string{"rule@example.com"},
			SentAt:     "2026-07-07T10:00:00+08:00",
			TextBody:   "实施服务能力认证考试还有五天",
			RawContent: "Subject: 认证考试倒计时\r\n\r\n实施服务能力认证考试还有五天",
		},
	}
	oldSyncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if oldSyncResp.Code != http.StatusOK {
		t.Fatalf("expected old sync status 200, got %d: %s", oldSyncResp.Code, oldSyncResp.Body.String())
	}

	historyRuleBody := `{
		"name":"培训通知归档",
		"enabled":true,
		"matchMode":"all",
		"targetFolderId":"` + otherFolderID + `",
		"sortOrder":20,
		"conditions":[
			{"field":"body","operator":"contains","value":"实施服务能力"}
		]
	}`
	historyRuleResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules", historyRuleBody, token)
	if historyRuleResp.Code != http.StatusCreated {
		t.Fatalf("expected history rule status 201, got %d: %s", historyRuleResp.Code, historyRuleResp.Body.String())
	}
	applyResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules/apply", `{"scope":"unfiled"}`, token)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("expected apply status 200, got %d: %s", applyResp.Code, applyResp.Body.String())
	}
	oldFilterResp := performRequest(router, http.MethodGet, "/api/v1/messages?folderId="+otherFolderID, "", token)
	if got := listSubjects(t, oldFilterResp.Body.Bytes()); !equalStringSlices(got, []string{"认证考试倒计时"}) {
		t.Fatalf("expected history rule message in folder, got %#v", got)
	}
}

func TestMailRuleDeleteRemovesRuleAndConditions(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "delete-rule-message",
				MessageID:  "<delete-rule-message@example.com>",
				Subject:    "网络安全整改通知",
				From:       "security@example.com",
				To:         []string{"first@example.com"},
				TextBody:   "请安装主机探针",
				RawContent: "Subject: 网络安全整改通知\r\n\r\n请安装主机探针",
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "delete-rule-user", "delete-rule-user@example.com")
	accountID := createTestAccount(t, router, token)

	folderResp := performRequest(router, http.MethodPost, "/api/v1/mail-folders", `{"name":"安全通知","color":"#1f66d1","sortOrder":10}`, token)
	if folderResp.Code != http.StatusCreated {
		t.Fatalf("expected folder status 201, got %d: %s", folderResp.Code, folderResp.Body.String())
	}
	folderID := nestedString(t, decodeEnvelope(t, folderResp.Body.Bytes()), "data", "id")
	ruleResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules", `{
		"name":"安全通知归档",
		"enabled":true,
		"matchMode":"all",
		"targetFolderId":"`+folderID+`",
		"sortOrder":10,
		"conditions":[{"field":"subject","operator":"contains","value":"网络安全"}]
	}`, token)
	if ruleResp.Code != http.StatusCreated {
		t.Fatalf("expected rule status 201, got %d: %s", ruleResp.Code, ruleResp.Body.String())
	}
	ruleID := nestedString(t, decodeEnvelope(t, ruleResp.Body.Bytes()), "data", "id")

	deleteResp := performRequest(router, http.MethodDelete, "/api/v1/mail-rules/"+ruleID, "", token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete rule status 200, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
	listResp := performRequest(router, http.MethodGet, "/api/v1/mail-rules", "", token)
	if listItemCount(t, listResp.Body.Bytes()) != 0 {
		t.Fatalf("expected no rules after delete, got %s", listResp.Body.String())
	}

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	applyResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules/apply", `{"scope":"all"}`, token)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("expected apply status 200, got %d: %s", applyResp.Code, applyResp.Body.String())
	}
	filterResp := performRequest(router, http.MethodGet, "/api/v1/messages?folderId="+folderID, "", token)
	if got := listSubjects(t, filterResp.Body.Bytes()); len(got) != 0 {
		t.Fatalf("expected deleted rule not to archive messages, got %#v", got)
	}
}

func TestUpdateMailRuleReplacesConditionsAndTargetFolder(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "security-message",
				MessageID:  "<security-message@example.com>",
				Subject:    "网络安全整改通知",
				From:       "security@example.com",
				To:         []string{"first@example.com"},
				TextBody:   "请安装主机探针",
				RawContent: "Subject: 网络安全整改通知\r\n\r\n请安装主机探针",
			},
			{
				UID:        "training-message",
				MessageID:  "<training-message@example.com>",
				Subject:    "认证考试倒计时",
				From:       "training@example.com",
				To:         []string{"first@example.com"},
				TextBody:   "实施服务能力认证考试还有五天",
				RawContent: "Subject: 认证考试倒计时\r\n\r\n实施服务能力认证考试还有五天",
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "update-rule-user", "update-rule-user@example.com")
	accountID := createTestAccount(t, router, token)

	securityFolderResp := performRequest(router, http.MethodPost, "/api/v1/mail-folders", `{"name":"安全通知","color":"#1f66d1","sortOrder":10}`, token)
	if securityFolderResp.Code != http.StatusCreated {
		t.Fatalf("expected security folder status 201, got %d: %s", securityFolderResp.Code, securityFolderResp.Body.String())
	}
	securityFolderID := nestedString(t, decodeEnvelope(t, securityFolderResp.Body.Bytes()), "data", "id")
	trainingFolderResp := performRequest(router, http.MethodPost, "/api/v1/mail-folders", `{"name":"培训通知","color":"#3b7a57","sortOrder":20}`, token)
	if trainingFolderResp.Code != http.StatusCreated {
		t.Fatalf("expected training folder status 201, got %d: %s", trainingFolderResp.Code, trainingFolderResp.Body.String())
	}
	trainingFolderID := nestedString(t, decodeEnvelope(t, trainingFolderResp.Body.Bytes()), "data", "id")

	ruleResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules", `{
		"name":"待更新规则",
		"enabled":true,
		"matchMode":"all",
		"targetFolderId":"`+securityFolderID+`",
		"sortOrder":10,
		"conditions":[{"field":"subject","operator":"contains","value":"网络安全"}]
	}`, token)
	if ruleResp.Code != http.StatusCreated {
		t.Fatalf("expected rule status 201, got %d: %s", ruleResp.Code, ruleResp.Body.String())
	}
	ruleID := nestedString(t, decodeEnvelope(t, ruleResp.Body.Bytes()), "data", "id")

	updateRuleResp := performRequest(router, http.MethodPut, "/api/v1/mail-rules/"+ruleID, `{
		"name":"培训通知归档",
		"enabled":true,
		"matchMode":"all",
		"targetFolderId":"`+trainingFolderID+`",
		"sortOrder":5,
		"conditions":[{"field":"from","operator":"contains","value":"training@example.com"}]
	}`, token)
	if updateRuleResp.Code != http.StatusOK {
		t.Fatalf("expected update rule status 200, got %d: %s", updateRuleResp.Code, updateRuleResp.Body.String())
	}
	ruleData := decodeEnvelope(t, updateRuleResp.Body.Bytes())["data"].(map[string]any)
	if ruleData["targetFolderId"] != trainingFolderID {
		t.Fatalf("expected updated target folder, got %#v", ruleData)
	}
	if conditions, ok := ruleData["conditions"].([]any); !ok || len(conditions) != 1 {
		t.Fatalf("expected one replacement condition, got %#v", ruleData["conditions"])
	}

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	securityFilterResp := performRequest(router, http.MethodGet, "/api/v1/messages?folderId="+securityFolderID, "", token)
	if got := listSubjects(t, securityFilterResp.Body.Bytes()); len(got) != 0 {
		t.Fatalf("expected old rule condition not to archive messages, got %#v", got)
	}
	trainingFilterResp := performRequest(router, http.MethodGet, "/api/v1/messages?folderId="+trainingFolderID, "", token)
	if got := listSubjects(t, trainingFilterResp.Body.Bytes()); !equalStringSlices(got, []string{"认证考试倒计时"}) {
		t.Fatalf("expected updated rule to archive training message, got %#v", got)
	}
}

func TestMicrosoftOAuthCreatesOAuthMailAccount(t *testing.T) {
	exchanger := &oauth.FakeMicrosoftExchanger{
		Token: oauth.Token{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
		Account: oauth.MicrosoftAccount{Email: "oauth@example.com"},
	}
	router := newTestRouterWithDependencies(t, true, &mail.FakeFetcher{}, exchanger)
	token := registerTestUser(t, router, "oauth-user", "oauth-user@example.com")

	startResp := performRequest(router, http.MethodPost, "/api/v1/oauth/microsoft/start", "", token)
	if startResp.Code != http.StatusOK {
		t.Fatalf("expected oauth start status 200, got %d: %s", startResp.Code, startResp.Body.String())
	}
	state := nestedString(t, decodeEnvelope(t, startResp.Body.Bytes()), "data", "state")
	authURL := nestedString(t, decodeEnvelope(t, startResp.Body.Bytes()), "data", "authUrl")
	if state == "" || authURL == "" {
		t.Fatalf("expected oauth state and authUrl, got %s", startResp.Body.String())
	}

	callbackResp := performRequest(router, http.MethodPost, "/api/v1/oauth/microsoft/complete", `{"code":"code-value","state":"`+state+`"}`, token)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("expected callback status 200, got %d: %s", callbackResp.Code, callbackResp.Body.String())
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/mail-accounts", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	if listItemCount(t, listResp.Body.Bytes()) != 1 {
		t.Fatalf("expected one oauth account, got %s", listResp.Body.String())
	}
	first := firstListItem(t, listResp.Body.Bytes())
	if first["authType"] != "oauth2" {
		t.Fatalf("expected authType oauth2, got %#v", first["authType"])
	}
	if first["provider"] != "microsoft" {
		t.Fatalf("expected provider microsoft, got %#v", first["provider"])
	}
}

func TestMicrosoftOAuthStartRequiresClientID(t *testing.T) {
	router := newTestRouterWithDependenciesAndConfig(t, true, &mail.FakeFetcher{}, &oauth.FakeMicrosoftExchanger{}, func(cfg *config.Config) {
		cfg.OAuth.Microsoft.ClientID = ""
	})
	token := registerTestUser(t, router, "missing-oauth", "missing-oauth@example.com")

	resp := performRequest(router, http.MethodPost, "/api/v1/oauth/microsoft/start", "", token)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected oauth start status 400, got %d: %s", resp.Code, resp.Body.String())
	}
	if decodeEnvelope(t, resp.Body.Bytes())["success"] != false {
		t.Fatalf("expected success false, got %s", resp.Body.String())
	}
}

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
