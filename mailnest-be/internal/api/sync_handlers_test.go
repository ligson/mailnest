package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"mailnest-be/internal/mail"
)

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
