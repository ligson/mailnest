package api

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"mailnest-be/internal/mail"
)

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

func TestMessageDetailDoesNotEmbedLargeUnsupportedInlineImage(t *testing.T) {
	largeImage := bytes.Repeat([]byte{0x01}, maxInlineImageTransformBytes+1)
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "large-inline-tiff",
				MessageID:  "<large-inline-tiff@example.com>",
				Subject:    "大内嵌图片",
				From:       "sender@example.com",
				To:         []string{"receiver@example.com"},
				HTMLBody:   `<p>截图如下</p><img src="cid:large-inline-image">`,
				RawContent: "Subject: 大内嵌图片\r\n\r\n截图如下",
				Attachments: []mail.FetchedAttachment{
					{
						Filename:    "large.tiff",
						ContentType: "image/tiff",
						ContentID:   "large-inline-image",
						Inline:      true,
						Data:        largeImage,
					},
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "large-inline-user", "large-inline@example.com")
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
	if strings.Contains(htmlBody, "cid:large-inline-image") || strings.Contains(htmlBody, "data:image/tiff;base64") {
		t.Fatalf("expected large inline image to be replaced without embedding original bytes, got %q", htmlBody)
	}
	if !strings.Contains(htmlBody, "data:image/svg+xml") || !strings.Contains(htmlBody, "%E5%86%85%E5%B5%8C%E5%9B%BE%E7%89%87%E8%BE%83%E5%A4%A7") {
		t.Fatalf("expected large inline image placeholder, got %q", htmlBody)
	}
	if len(detailResp.Body.Bytes()) > maxInlineImageTransformBytes/2 {
		t.Fatalf("expected compact detail response, got %d bytes", len(detailResp.Body.Bytes()))
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
