package api

import (
	"net/http"
	"testing"

	"mailnest-be/internal/mail"
)

func TestMailThreadsGroupRepliesAndRespectAccountBoundary(t *testing.T) {
	fetcher := &mail.FakeFetcher{}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "thread-user", "thread-user@example.com")
	accountID := createTestAccount(t, router, token)

	fetcher.Messages = []mail.FetchedMessage{
		{
			UID:        "thread-root",
			MessageID:  "<thread-root@example.com>",
			Subject:    "季度计划",
			From:       "sender@example.com",
			To:         []string{"thread-user@example.com"},
			SentAt:     "2026-07-01T10:00:00+08:00",
			TextBody:   "第一封正文",
			RawContent: "Subject: 季度计划\r\nMessage-ID: <thread-root@example.com>\r\n\r\n第一封正文",
		},
		{
			UID:        "thread-reply",
			MessageID:  "<thread-reply@example.com>",
			InReplyTo:  "<thread-root@example.com>",
			References: "<thread-root@example.com>",
			Subject:    "Re: 季度计划",
			From:       "thread-user@example.com",
			To:         []string{"sender@example.com"},
			SentAt:     "2026-07-01T10:05:00+08:00",
			TextBody:   "回复正文",
			RawContent: "Subject: Re: 季度计划\r\nIn-Reply-To: <thread-root@example.com>\r\n\r\n回复正文",
		},
	}
	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	threadsResp := performRequest(router, http.MethodGet, "/api/v1/threads?systemFolder=inbox", "", token)
	if threadsResp.Code != http.StatusOK {
		t.Fatalf("expected thread list status 200, got %d: %s", threadsResp.Code, threadsResp.Body.String())
	}
	thread := firstListItem(t, threadsResp.Body.Bytes())
	threadID := thread["id"].(string)
	if thread["messageCount"] != float64(2) || thread["subject"] != "季度计划" {
		t.Fatalf("expected grouped thread with two messages, got %#v", thread)
	}
	latest := thread["latestMessage"].(map[string]any)
	if latest["threadId"] != threadID || latest["subject"] != "Re: 季度计划" {
		t.Fatalf("expected latest message to reference thread, got %#v", latest)
	}

	detailResp := performRequest(router, http.MethodGet, "/api/v1/threads/"+threadID, "", token)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected thread detail status 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	detail := decodeEnvelope(t, detailResp.Body.Bytes())["data"].(map[string]any)
	messages := detail["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected two thread messages, got %#v", messages)
	}
	for _, raw := range messages {
		message := raw.(map[string]any)
		if message["threadId"] != threadID {
			t.Fatalf("expected message to reference thread %s, got %#v", threadID, message)
		}
	}

	secondAccountResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts", `{
		"displayName":"备用邮箱",
		"email":"second-thread@example.com",
		"imapHost":"imap.example.com",
		"imapPort":993,
		"imapTls":true,
		"imapUsername":"second-thread@example.com",
		"imapPassword":"mail-password",
		"sentFolder":"Sent",
		"pollIntervalMinutes":10,
		"enabled":true
	}`, token)
	if secondAccountResp.Code != http.StatusCreated {
		t.Fatalf("expected second account status 201, got %d: %s", secondAccountResp.Code, secondAccountResp.Body.String())
	}
	secondAccountID := nestedString(t, decodeEnvelope(t, secondAccountResp.Body.Bytes()), "data", "id")
	fetcher.Messages = []mail.FetchedMessage{
		{
			UID:        "thread-same-subject-other-account",
			MessageID:  "<thread-same-subject-other-account@example.com>",
			Subject:    "Re: 季度计划",
			From:       "sender@example.com",
			To:         []string{"second-thread@example.com"},
			SentAt:     "2026-07-01T10:10:00+08:00",
			TextBody:   "另一个邮箱里的同主题邮件",
			RawContent: "Subject: Re: 季度计划\r\n\r\n另一个邮箱里的同主题邮件",
		},
	}
	secondSyncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+secondAccountID+"/sync", "", token)
	if secondSyncResp.Code != http.StatusOK {
		t.Fatalf("expected second sync status 200, got %d: %s", secondSyncResp.Code, secondSyncResp.Body.String())
	}
	allThreadsResp := performRequest(router, http.MethodGet, "/api/v1/threads?systemFolder=inbox", "", token)
	if allThreadsResp.Code != http.StatusOK {
		t.Fatalf("expected all threads status 200, got %d: %s", allThreadsResp.Code, allThreadsResp.Body.String())
	}
	if got := listItemCount(t, allThreadsResp.Body.Bytes()); got != 2 {
		t.Fatalf("expected same subject across accounts to stay separate, got %d threads: %s", got, allThreadsResp.Body.String())
	}

	rebuildResp := performRequest(router, http.MethodPost, "/api/v1/threads/rebuild", `{"scope":"all"}`, token)
	if rebuildResp.Code != http.StatusOK {
		t.Fatalf("expected rebuild status 200, got %d: %s", rebuildResp.Code, rebuildResp.Body.String())
	}
	rebuildData := decodeEnvelope(t, rebuildResp.Body.Bytes())["data"].(map[string]any)
	if rebuildData["processedCount"] != float64(3) || rebuildData["threadCount"] != float64(2) {
		t.Fatalf("expected rebuild to process three messages into two threads, got %#v", rebuildData)
	}
}

func TestMailRuleLogsExposeRuleStatsAndMessageHistory(t *testing.T) {
	fetcher := &mail.FakeFetcher{}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "rule-log-user", "rule-log-user@example.com")
	accountID := createTestAccount(t, router, token)

	ruleResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules", `{
		"name":"广告邮件标记垃圾",
		"enabled":true,
		"matchMode":"all",
		"actionType":"mark_spam",
		"sortOrder":10,
		"conditions":[
			{"field":"subject","operator":"contains","value":"优惠"},
			{"field":"from","operator":"contains","value":"promo@example.com"}
		]
	}`, token)
	if ruleResp.Code != http.StatusCreated {
		t.Fatalf("expected rule status 201, got %d: %s", ruleResp.Code, ruleResp.Body.String())
	}
	ruleID := nestedString(t, decodeEnvelope(t, ruleResp.Body.Bytes()), "data", "id")

	fetcher.Messages = []mail.FetchedMessage{
		{
			UID:        "rule-log-message",
			MessageID:  "<rule-log-message@example.com>",
			Subject:    "限时优惠提醒",
			From:       "promo@example.com",
			To:         []string{"rule-log-user@example.com"},
			SentAt:     "2026-07-06T10:00:00+08:00",
			TextBody:   "广告内容",
			RawContent: "Subject: 限时优惠提醒\r\n\r\n广告内容",
		},
	}
	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	spamResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=spam", "", token)
	if spamResp.Code != http.StatusOK {
		t.Fatalf("expected spam list status 200, got %d: %s", spamResp.Code, spamResp.Body.String())
	}
	messageID := firstListItemID(t, spamResp.Body.Bytes())

	messageLogsResp := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID+"/rule-logs", "", token)
	if messageLogsResp.Code != http.StatusOK {
		t.Fatalf("expected message rule logs status 200, got %d: %s", messageLogsResp.Code, messageLogsResp.Body.String())
	}
	messageLog := firstListItem(t, messageLogsResp.Body.Bytes())
	if messageLog["ruleId"] != ruleID || messageLog["ruleName"] != "广告邮件标记垃圾" || messageLog["triggerType"] != "sync" || messageLog["resultStatus"] != "applied" {
		t.Fatalf("expected applied sync rule log, got %#v", messageLog)
	}

	ruleLogsResp := performRequest(router, http.MethodGet, "/api/v1/rule-logs?ruleId="+ruleID, "", token)
	if ruleLogsResp.Code != http.StatusOK {
		t.Fatalf("expected rule logs status 200, got %d: %s", ruleLogsResp.Code, ruleLogsResp.Body.String())
	}
	if got := listItemCount(t, ruleLogsResp.Body.Bytes()); got != 1 {
		t.Fatalf("expected one rule log by rule id, got %d: %s", got, ruleLogsResp.Body.String())
	}

	rulesResp := performRequest(router, http.MethodGet, "/api/v1/mail-rules", "", token)
	if rulesResp.Code != http.StatusOK {
		t.Fatalf("expected rule list status 200, got %d: %s", rulesResp.Code, rulesResp.Body.String())
	}
	rule := firstListItem(t, rulesResp.Body.Bytes())
	if rule["hitCount"] != float64(1) || rule["lastResult"] != "applied" || rule["lastHitAt"] == nil {
		t.Fatalf("expected rule hit stats, got %#v", rule)
	}
}
