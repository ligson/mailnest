package api

import (
	"net/http"
	"testing"

	"mailnest-be/internal/mail"
)

func TestBatchActionsAttachmentsAndSyncJobsAreAvailableAndIsolated(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "batch-1",
				MessageID:  "<batch-1@example.com>",
				Subject:    "带附件批量邮件",
				From:       "sender@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-19T10:00:00+08:00",
				TextBody:   "批量操作测试",
				RawContent: "Subject: 带附件批量邮件\r\n\r\n批量操作测试",
				Attachments: []mail.FetchedAttachment{
					{
						Filename:    "report.pdf",
						ContentType: "application/pdf",
						Data:        []byte("%PDF-batch"),
					},
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	firstToken := registerTestUser(t, router, "batch-first", "batch-first@example.com")
	secondToken := registerTestUser(t, router, "batch-second", "batch-second@example.com")
	accountID := createTestAccount(t, router, firstToken)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", firstToken)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	jobID := nestedString(t, decodeEnvelope(t, syncResp.Body.Bytes()), "data", "jobId")

	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", firstToken)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected messages status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	messageID := firstListItemID(t, listResp.Body.Bytes())

	batchResp := performRequest(router, http.MethodPost, "/api/v1/messages/batch-actions", `{
		"messageIds":["`+messageID+`"],
		"action":"star"
	}`, firstToken)
	if batchResp.Code != http.StatusOK {
		t.Fatalf("expected batch star status 200, got %d: %s", batchResp.Code, batchResp.Body.String())
	}

	starredResp := performRequest(router, http.MethodGet, "/api/v1/messages?starred=true", "", firstToken)
	if starredResp.Code != http.StatusOK || listItemCount(t, starredResp.Body.Bytes()) != 1 {
		t.Fatalf("expected one starred message, got %d %s", starredResp.Code, starredResp.Body.String())
	}

	spamResp := performRequest(router, http.MethodPost, "/api/v1/messages/batch-actions", `{
		"messageIds":["`+messageID+`"],
		"action":"mark_spam"
	}`, firstToken)
	if spamResp.Code != http.StatusOK {
		t.Fatalf("expected batch spam status 200, got %d: %s", spamResp.Code, spamResp.Body.String())
	}
	spamFolderResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=spam", "", firstToken)
	if spamFolderResp.Code != http.StatusOK || listItemCount(t, spamFolderResp.Body.Bytes()) != 1 {
		t.Fatalf("expected one spam message, got %d %s", spamFolderResp.Code, spamFolderResp.Body.String())
	}
	defaultListResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", firstToken)
	if defaultListResp.Code != http.StatusOK || listItemCount(t, defaultListResp.Body.Bytes()) != 0 {
		t.Fatalf("expected spam message to leave default list, got %d %s", defaultListResp.Code, defaultListResp.Body.String())
	}
	unspamResp := performRequest(router, http.MethodPost, "/api/v1/messages/batch-actions", `{
		"messageIds":["`+messageID+`"],
		"action":"unmark_spam"
	}`, firstToken)
	if unspamResp.Code != http.StatusOK {
		t.Fatalf("expected batch unspam status 200, got %d: %s", unspamResp.Code, unspamResp.Body.String())
	}
	clearedSpamResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=spam", "", firstToken)
	if clearedSpamResp.Code != http.StatusOK || listItemCount(t, clearedSpamResp.Body.Bytes()) != 0 {
		t.Fatalf("expected spam folder to be empty after unmark, got %d %s", clearedSpamResp.Code, clearedSpamResp.Body.String())
	}

	deleteResp := performRequest(router, http.MethodPost, "/api/v1/messages/batch-actions", `{
		"messageIds":["`+messageID+`"],
		"action":"delete"
	}`, firstToken)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected batch delete status 200, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}

	trashResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=trash", "", firstToken)
	if trashResp.Code != http.StatusOK || listItemCount(t, trashResp.Body.Bytes()) != 1 {
		t.Fatalf("expected one trash message, got %d %s", trashResp.Code, trashResp.Body.String())
	}

	attachmentResp := performRequest(router, http.MethodGet, "/api/v1/attachments", "", firstToken)
	if attachmentResp.Code != http.StatusOK || listItemCount(t, attachmentResp.Body.Bytes()) != 1 {
		t.Fatalf("expected one attachment, got %d %s", attachmentResp.Code, attachmentResp.Body.String())
	}
	secondAttachmentResp := performRequest(router, http.MethodGet, "/api/v1/attachments", "", secondToken)
	if secondAttachmentResp.Code != http.StatusOK || listItemCount(t, secondAttachmentResp.Body.Bytes()) != 0 {
		t.Fatalf("expected second user to see no attachments, got %d %s", secondAttachmentResp.Code, secondAttachmentResp.Body.String())
	}

	jobsResp := performRequest(router, http.MethodGet, "/api/v1/sync-jobs", "", firstToken)
	if jobsResp.Code != http.StatusOK || listItemCount(t, jobsResp.Body.Bytes()) == 0 {
		t.Fatalf("expected sync jobs for first user, got %d %s", jobsResp.Code, jobsResp.Body.String())
	}
	eventsResp := performRequest(router, http.MethodGet, "/api/v1/sync-jobs/"+jobID+"/events", "", firstToken)
	if eventsResp.Code != http.StatusOK || listItemCount(t, eventsResp.Body.Bytes()) == 0 {
		t.Fatalf("expected sync job events, got %d %s", eventsResp.Code, eventsResp.Body.String())
	}
	secondEventsResp := performRequest(router, http.MethodGet, "/api/v1/sync-jobs/"+jobID+"/events", "", secondToken)
	if secondEventsResp.Code != http.StatusInternalServerError && secondEventsResp.Code != http.StatusNotFound && secondEventsResp.Code != http.StatusOK {
		t.Fatalf("unexpected second user sync event status %d: %s", secondEventsResp.Code, secondEventsResp.Body.String())
	}
	if secondEventsResp.Code == http.StatusOK && listItemCount(t, secondEventsResp.Body.Bytes()) != 0 {
		t.Fatalf("expected second user to see no sync events, got %s", secondEventsResp.Body.String())
	}
}
