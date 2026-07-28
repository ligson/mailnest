package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"mailnest-be/internal/mail"
)

func TestAttachmentPreviewServesInlineContentAndIsIsolated(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "preview-attachment-1",
				MessageID:  "<preview-attachment-1@example.com>",
				Subject:    "预览附件邮件",
				From:       "sender@example.com",
				To:         []string{"first@example.com"},
				SentAt:     "2026-07-27T10:00:00+08:00",
				TextBody:   "附件预览测试",
				RawContent: "Subject: 预览附件邮件\r\n\r\n附件预览测试",
				Attachments: []mail.FetchedAttachment{
					{
						Filename:    "preview.txt",
						ContentType: "text/plain; charset=utf-8",
						Data:        []byte("hello preview"),
					},
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	firstToken := registerTestUser(t, router, "preview-first", "preview-first@example.com")
	secondToken := registerTestUser(t, router, "preview-second", "preview-second@example.com")
	accountID := createTestAccount(t, router, firstToken)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", firstToken)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	attachmentResp := performRequest(router, http.MethodGet, "/api/v1/attachments", "", firstToken)
	if attachmentResp.Code != http.StatusOK {
		t.Fatalf("expected attachments status 200, got %d: %s", attachmentResp.Code, attachmentResp.Body.String())
	}
	attachmentID := firstListItemID(t, attachmentResp.Body.Bytes())

	previewResp := performRequest(router, http.MethodGet, "/api/v1/attachments/"+attachmentID+"/preview", "", firstToken)
	if previewResp.Code != http.StatusOK {
		t.Fatalf("expected preview status 200, got %d: %s", previewResp.Code, previewResp.Body.String())
	}
	if got := previewResp.Body.String(); got != "hello preview" {
		t.Fatalf("expected preview body, got %q", got)
	}
	if disposition := previewResp.Header().Get("Content-Disposition"); disposition == "" || !bytes.Contains([]byte(disposition), []byte("inline")) {
		t.Fatalf("expected inline content disposition, got %q", disposition)
	}

	isolatedResp := performRequest(router, http.MethodGet, "/api/v1/attachments/"+attachmentID+"/preview", "", secondToken)
	if isolatedResp.Code != http.StatusNotFound {
		t.Fatalf("expected isolated preview status 404, got %d: %s", isolatedResp.Code, isolatedResp.Body.String())
	}
}

func TestAttachmentPreviewInfersPDFContentTypeFromFilename(t *testing.T) {
	fetcher := &mail.FakeFetcher{
		Messages: []mail.FetchedMessage{
			{
				UID:        "preview-pdf-attachment",
				MessageID:  "<preview-pdf-attachment@example.com>",
				Subject:    "PDF 预览附件邮件",
				From:       "sender@example.com",
				To:         []string{"reader@example.com"},
				SentAt:     "2026-07-27T11:00:00+08:00",
				TextBody:   "PDF 附件预览测试",
				RawContent: "Subject: PDF 预览附件邮件\r\n\r\nPDF 附件预览测试",
				Attachments: []mail.FetchedAttachment{
					{
						Filename:    "report.pdf",
						ContentType: "application/octet-stream",
						Data:        []byte("%PDF-1.7"),
					},
				},
			},
		},
	}
	router := newTestRouterWithFetcher(t, true, fetcher)
	token := registerTestUser(t, router, "preview-pdf", "preview-pdf@example.com")
	accountID := createTestAccount(t, router, token)

	syncResp := performRequest(router, http.MethodPost, "/api/v1/mail-accounts/"+accountID+"/sync", "", token)
	if syncResp.Code != http.StatusOK {
		t.Fatalf("expected sync status 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	attachmentResp := performRequest(router, http.MethodGet, "/api/v1/attachments", "", token)
	if attachmentResp.Code != http.StatusOK {
		t.Fatalf("expected attachments status 200, got %d: %s", attachmentResp.Code, attachmentResp.Body.String())
	}
	attachmentID := firstListItemID(t, attachmentResp.Body.Bytes())

	previewResp := performRequest(router, http.MethodGet, "/api/v1/attachments/"+attachmentID+"/preview", "", token)
	if previewResp.Code != http.StatusOK {
		t.Fatalf("expected preview status 200, got %d: %s", previewResp.Code, previewResp.Body.String())
	}
	if got := previewResp.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("expected inferred PDF content type, got %q", got)
	}
	if disposition := previewResp.Header().Get("Content-Disposition"); disposition == "" || !bytes.Contains([]byte(disposition), []byte("inline")) {
		t.Fatalf("expected inline content disposition, got %q", disposition)
	}
}

func TestImportEMLMessageCreatesMessageAndDeduplicates(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "eml-import", "eml-import@example.com")
	accountID := createTestAccount(t, router, token)
	raw := []byte("From: Sender <sender@example.com>\r\nTo: Reader <reader@example.com>\r\nSubject: Imported EML\r\nMessage-ID: <imported-eml@example.com>\r\nDate: Mon, 27 Jul 2026 10:00:00 +0800\r\n\r\nHello from eml.")

	importResp := performEMLImport(t, router, token, accountID, raw)
	if importResp.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d: %s", importResp.Code, importResp.Body.String())
	}
	data := decodeEnvelope(t, importResp.Body.Bytes())["data"].(map[string]any)
	if data["inserted"] != true {
		t.Fatalf("expected first import to insert, got %#v", data["inserted"])
	}
	messagePayload := data["message"].(map[string]any)
	messageID := messagePayload["id"].(string)
	if messagePayload["subject"] != "Imported EML" {
		t.Fatalf("expected imported subject, got %#v", messagePayload["subject"])
	}

	detailResp := performRequest(router, http.MethodGet, "/api/v1/messages/"+messageID, "", token)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	detail := decodeEnvelope(t, detailResp.Body.Bytes())["data"].(map[string]any)
	if detail["textBody"] != "Hello from eml." {
		t.Fatalf("expected imported body, got %#v", detail["textBody"])
	}

	duplicateResp := performEMLImport(t, router, token, accountID, raw)
	if duplicateResp.Code != http.StatusOK {
		t.Fatalf("expected duplicate import status 200, got %d: %s", duplicateResp.Code, duplicateResp.Body.String())
	}
	duplicateData := decodeEnvelope(t, duplicateResp.Body.Bytes())["data"].(map[string]any)
	if duplicateData["inserted"] != false {
		t.Fatalf("expected duplicate import to be deduplicated, got %#v", duplicateData["inserted"])
	}
	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", token)
	if listResp.Code != http.StatusOK || listItemCount(t, listResp.Body.Bytes()) != 1 {
		t.Fatalf("expected one imported message, got %d %s", listResp.Code, listResp.Body.String())
	}
}

func TestImportEMLMessageBatchContinuesWhenOneFileFails(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "eml-batch", "eml-batch@example.com")
	accountID := createTestAccount(t, router, token)
	first := []byte("From: Sender <sender@example.com>\r\nTo: Reader <reader@example.com>\r\nSubject: Batch One\r\nMessage-ID: <batch-one@example.com>\r\nDate: Mon, 27 Jul 2026 10:00:00 +0800\r\n\r\nOne.")
	second := []byte("From: Sender <sender@example.com>\r\nTo: Reader <reader@example.com>\r\nSubject: Batch Two\r\nMessage-ID: <batch-two@example.com>\r\nDate: Mon, 27 Jul 2026 10:01:00 +0800\r\n\r\nTwo.")

	resp := performEMLImportFiles(t, router, token, accountID, []emlImportFile{
		{Name: "one.eml", Data: first},
		{Name: "bad.txt", Data: []byte("not eml")},
		{Name: "two.eml", Data: second},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected batch import status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	data := decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)
	if data["total"] != float64(3) || data["successCount"] != float64(2) || data["insertedCount"] != float64(2) || data["failedCount"] != float64(1) {
		t.Fatalf("expected batch summary, got %#v", data)
	}
	items := data["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("expected three item results, got %#v", items)
	}
	failed := items[1].(map[string]any)
	if failed["success"] != false || failed["error"] == nil {
		t.Fatalf("expected second item to fail only itself, got %#v", failed)
	}
	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", token)
	if listResp.Code != http.StatusOK || listItemCount(t, listResp.Body.Bytes()) != 2 {
		t.Fatalf("expected two imported messages, got %d %s", listResp.Code, listResp.Body.String())
	}
}

func TestImportEMLMessageBatchDoesNotLimitFileCount(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "eml-many", "eml-many@example.com")
	accountID := createTestAccount(t, router, token)
	files := make([]emlImportFile, 0, 101)
	for i := 0; i < 101; i++ {
		raw := []byte(fmt.Sprintf("From: Sender <sender@example.com>\r\nTo: Reader <reader@example.com>\r\nSubject: Batch Many %03d\r\nMessage-ID: <batch-many-%03d@example.com>\r\nDate: Mon, 27 Jul 2026 10:00:00 +0800\r\n\r\nMany %03d.", i, i, i))
		files = append(files, emlImportFile{Name: fmt.Sprintf("many-%03d.eml", i), Data: raw})
	}

	resp := performEMLImportFiles(t, router, token, accountID, files)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected many-file import status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	data := decodeEnvelope(t, resp.Body.Bytes())["data"].(map[string]any)
	if data["total"] != float64(101) || data["successCount"] != float64(101) || data["insertedCount"] != float64(101) || data["failedCount"] != float64(0) {
		t.Fatalf("expected all 101 EML files to import, got %#v", data)
	}
}

func TestImportEMLResumableUploadCanResumeAndFinish(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "eml-resume", "eml-resume@example.com")
	accountID := createTestAccount(t, router, token)
	raw := []byte("From: Sender <sender@example.com>\r\nTo: Reader <reader@example.com>\r\nSubject: Resumable EML\r\nMessage-ID: <resumable-eml@example.com>\r\nDate: Mon, 27 Jul 2026 10:00:00 +0800\r\n\r\nHello resumable eml.")
	createBody := fmt.Sprintf(`{"accountId":%q,"folder":"INBOX","filename":"resume.eml","size":%d,"lastModified":1785200000000,"fileKey":"resume.eml|%d|1785200000000"}`, accountID, len(raw), len(raw))

	createResp := performRequest(router, http.MethodPost, "/api/v1/messages/import-eml/uploads", createBody, token)
	if createResp.Code != http.StatusOK {
		t.Fatalf("expected create upload status 200, got %d: %s", createResp.Code, createResp.Body.String())
	}
	createData := decodeEnvelope(t, createResp.Body.Bytes())["data"].(map[string]any)
	uploadID := createData["uploadId"].(string)
	if createData["uploadedBytes"] != float64(0) {
		t.Fatalf("expected empty upload session, got %#v", createData)
	}

	firstChunk := raw[:42]
	firstResp := performBinaryRequest(router, http.MethodPut, "/api/v1/messages/import-eml/uploads/"+uploadID+"/chunk", firstChunk, "0", token)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("expected first chunk status 200, got %d: %s", firstResp.Code, firstResp.Body.String())
	}
	firstData := decodeEnvelope(t, firstResp.Body.Bytes())["data"].(map[string]any)
	if firstData["uploadedBytes"] != float64(len(firstChunk)) {
		t.Fatalf("expected first uploaded bytes, got %#v", firstData)
	}

	resumeResp := performRequest(router, http.MethodPost, "/api/v1/messages/import-eml/uploads", createBody, token)
	if resumeResp.Code != http.StatusOK {
		t.Fatalf("expected resume create status 200, got %d: %s", resumeResp.Code, resumeResp.Body.String())
	}
	resumeData := decodeEnvelope(t, resumeResp.Body.Bytes())["data"].(map[string]any)
	if resumeData["uploadId"] != uploadID || resumeData["uploadedBytes"] != float64(len(firstChunk)) {
		t.Fatalf("expected resumed offset, got %#v", resumeData)
	}

	conflictResp := performBinaryRequest(router, http.MethodPut, "/api/v1/messages/import-eml/uploads/"+uploadID+"/chunk", []byte("bad-offset"), "0", token)
	if conflictResp.Code != http.StatusConflict {
		t.Fatalf("expected offset conflict status 409, got %d: %s", conflictResp.Code, conflictResp.Body.String())
	}
	conflictData := decodeEnvelope(t, conflictResp.Body.Bytes())["data"].(map[string]any)
	if conflictData["uploadedBytes"] != float64(len(firstChunk)) {
		t.Fatalf("expected server offset in conflict payload, got %#v", conflictData)
	}

	restResp := performBinaryRequest(router, http.MethodPut, "/api/v1/messages/import-eml/uploads/"+uploadID+"/chunk", raw[len(firstChunk):], fmt.Sprint(len(firstChunk)), token)
	if restResp.Code != http.StatusOK {
		t.Fatalf("expected rest chunk status 200, got %d: %s", restResp.Code, restResp.Body.String())
	}
	finishResp := performRequest(router, http.MethodPost, "/api/v1/messages/import-eml/uploads/"+uploadID+"/finish", `{}`, token)
	if finishResp.Code != http.StatusOK {
		t.Fatalf("expected finish status 200, got %d: %s", finishResp.Code, finishResp.Body.String())
	}
	finishData := decodeEnvelope(t, finishResp.Body.Bytes())["data"].(map[string]any)
	if finishData["inserted"] != true || finishData["uploadedBytes"] != float64(len(raw)) {
		t.Fatalf("expected inserted finish payload, got %#v", finishData)
	}

	secondFinishResp := performRequest(router, http.MethodPost, "/api/v1/messages/import-eml/uploads/"+uploadID+"/finish", `{}`, token)
	if secondFinishResp.Code != http.StatusOK {
		t.Fatalf("expected idempotent finish status 200, got %d: %s", secondFinishResp.Code, secondFinishResp.Body.String())
	}
	listResp := performRequest(router, http.MethodGet, "/api/v1/messages", "", token)
	if listResp.Code != http.StatusOK || listItemCount(t, listResp.Body.Bytes()) != 1 {
		t.Fatalf("expected one imported message, got %d %s", listResp.Code, listResp.Body.String())
	}
}

type emlImportFile struct {
	Name string
	Data []byte
}

func performEMLImport(t *testing.T, router http.Handler, token, accountID string, raw []byte) *httptest.ResponseRecorder {
	return performEMLImportFiles(t, router, token, accountID, []emlImportFile{{Name: "import.eml", Data: raw}})
}

func performEMLImportFiles(t *testing.T, router http.Handler, token, accountID string, files []emlImportFile) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("accountId", accountID); err != nil {
		t.Fatalf("write accountId: %v", err)
	}
	if err := writer.WriteField("folder", "INBOX"); err != nil {
		t.Fatalf("write folder: %v", err)
	}
	for _, item := range files {
		file, err := writer.CreateFormFile("file", item.Name)
		if err != nil {
			t.Fatalf("create eml form file: %v", err)
		}
		if _, err := file.Write(item.Data); err != nil {
			t.Fatalf("write eml file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return performMultipartRequest(router, http.MethodPost, "/api/v1/messages/import-eml", body.Bytes(), writer.FormDataContentType(), token)
}

func performBinaryRequest(handler http.Handler, method, path string, body []byte, offset, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Upload-Offset", offset)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
