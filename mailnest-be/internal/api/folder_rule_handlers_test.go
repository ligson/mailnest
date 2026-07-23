package api

import (
	"net/http"
	"testing"

	"mailnest-be/internal/mail"
)

func TestMailFoldersCanBeUpdatedAndReportRuleLinks(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "folder-edit", "folder-edit@example.com")

	createResp := performRequest(router, http.MethodPost, "/api/v1/mail-folders", `{
		"name":"安全通知",
		"color":"#1f66d1",
		"sortOrder":10
	}`, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected create folder status 201, got %d: %s", createResp.Code, createResp.Body.String())
	}
	folderID := nestedString(t, decodeEnvelope(t, createResp.Body.Bytes()), "data", "id")

	updateResp := performRequest(router, http.MethodPut, "/api/v1/mail-folders/"+folderID, `{
		"name":"安全与登录",
		"color":"#dc2626",
		"sortOrder":20
	}`, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update folder status 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}
	updated := decodeEnvelope(t, updateResp.Body.Bytes())["data"].(map[string]any)
	if updated["name"] != "安全与登录" || updated["color"] != "#dc2626" || updated["sortOrder"] != float64(20) {
		t.Fatalf("expected updated folder payload, got %#v", updated)
	}

	ruleResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules", `{
		"name":"登录提醒归档",
		"enabled":true,
		"matchMode":"all",
		"targetFolderId":"`+folderID+`",
		"sortOrder":10,
		"conditions":[{"field":"subject","operator":"contains","value":"登录"}]
	}`, token)
	if ruleResp.Code != http.StatusCreated {
		t.Fatalf("expected create rule status 201, got %d: %s", ruleResp.Code, ruleResp.Body.String())
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/mail-folders", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected list folder status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	items := decodeEnvelope(t, listResp.Body.Bytes())["data"].(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one folder, got %#v", items)
	}
	listed := items[0].(map[string]any)
	if listed["name"] != "安全与登录" || listed["ruleCount"] != float64(1) {
		t.Fatalf("expected folder to report rule count, got %#v", listed)
	}

	deleteResp := performRequest(router, http.MethodDelete, "/api/v1/mail-folders/"+folderID, "", token)
	if deleteResp.Code != http.StatusConflict {
		t.Fatalf("expected linked folder delete status 409, got %d: %s", deleteResp.Code, deleteResp.Body.String())
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

func TestMailRulesMarkSpamNewAndExistingMessages(t *testing.T) {
	fetcher := &mail.FakeFetcher{}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "spam-rule-user", "spam-rule-user@example.com")
	accountID := createTestAccount(t, router, token)

	fetcher.Messages = []mail.FetchedMessage{
		{
			UID:        "spam-old-message",
			MessageID:  "<spam-old-message@example.com>",
			Subject:    "优惠专享到货",
			From:       "promo@example.com",
			To:         []string{"rule@example.com"},
			SentAt:     "2026-07-07T10:00:00+08:00",
			TextBody:   "历史广告内容",
			RawContent: "Subject: 优惠专享到货\r\n\r\n历史广告内容",
		},
	}
	initialSyncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if initialSyncResp.Code != http.StatusOK {
		t.Fatalf("expected initial sync status 200, got %d: %s", initialSyncResp.Code, initialSyncResp.Body.String())
	}
	initialSpamResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=spam", "", token)
	if got := listSubjects(t, initialSpamResp.Body.Bytes()); len(got) != 0 {
		t.Fatalf("expected no spam before rule creation, got %#v", got)
	}

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
		t.Fatalf("expected spam rule status 201, got %d: %s", ruleResp.Code, ruleResp.Body.String())
	}

	fetcher.Messages = []mail.FetchedMessage{
		{
			UID:        "spam-new-message",
			MessageID:  "<spam-new-message@example.com>",
			Subject:    "限时优惠提醒",
			From:       "promo@example.com",
			To:         []string{"rule@example.com"},
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
	if got := listSubjects(t, spamResp.Body.Bytes()); !equalStringSlices(got, []string{"限时优惠提醒"}) {
		t.Fatalf("expected new spam message in spam folder, got %#v", got)
	}
	applyResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules/apply", `{"scope":"all"}`, token)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("expected spam apply status 200, got %d: %s", applyResp.Code, applyResp.Body.String())
	}
	afterApplyResp := performRequest(router, http.MethodGet, "/api/v1/messages?systemFolder=spam", "", token)
	if got := listSubjects(t, afterApplyResp.Body.Bytes()); !equalStringSlices(got, []string{"优惠专享到货", "限时优惠提醒"}) {
		t.Fatalf("expected historical spam message in spam folder, got %#v", got)
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

func TestPreviewMailRuleMatchesEnhancedConditions(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "preview-1",
				MessageID:  "<preview-1@example.com>",
				Subject:    "周报附件",
				From:       "boss@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-19T09:00:00+08:00",
				TextBody:   "请查收本周周报",
				RawContent: "Subject: 周报附件\r\n\r\n请查收本周周报",
				Attachments: []mail.FetchedAttachment{
					{
						Filename:    "weekly-report.pdf",
						ContentType: "application/pdf",
						Data:        []byte("%PDF-preview"),
					},
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "preview-user", "preview-user@example.com")
	accountID := createTestAccount(t, router, token)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	folderResp := performRequest(router, http.MethodPost, "/api/v1/mail-folders", `{
		"name":"附件归档",
		"color":"#1f66d1",
		"sortOrder":10
	}`, token)
	if folderResp.Code != http.StatusCreated {
		t.Fatalf("expected create folder status 201, got %d: %s", folderResp.Code, folderResp.Body.String())
	}
	folderID := nestedString(t, decodeEnvelope(t, folderResp.Body.Bytes()), "data", "id")

	previewResp := performRequest(router, http.MethodPost, "/api/v1/mail-rules/preview", `{
		"name":"附件周报预览",
		"enabled":true,
		"matchMode":"all",
		"priority":10,
		"stopOnMatch":true,
		"actionType":"move_folder",
		"targetFolderId":"`+folderID+`",
		"sortOrder":10,
		"limit":5,
		"conditions":[
			{"field":"has_attachments","operator":"is_true","value":""},
			{"field":"attachment_filename","operator":"contains","value":"weekly-report"}
		]
	}`, token)
	if previewResp.Code != http.StatusOK {
		t.Fatalf("expected rule preview status 200, got %d: %s", previewResp.Code, previewResp.Body.String())
	}
	data := decodeEnvelope(t, previewResp.Body.Bytes())["data"].(map[string]any)
	if data["matchedCount"] != float64(1) {
		t.Fatalf("expected one preview match, got %#v", data)
	}
	samples, ok := data["samples"].([]any)
	if !ok || len(samples) != 1 {
		t.Fatalf("expected one preview sample, got %#v", data["samples"])
	}
	sample := samples[0].(map[string]any)
	if sample["subject"] != "周报附件" {
		t.Fatalf("expected preview sample subject, got %#v", sample)
	}
}
