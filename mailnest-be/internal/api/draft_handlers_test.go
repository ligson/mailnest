package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestDraftHandlersSaveListUpdateAndDelete(t *testing.T) {
	router := newTestRouter(t, true)
	token := registerTestUser(t, router, "draft-api-user", "draft-api-user@example.com")
	accountID := createTestAccount(t, router, token)

	createBody := `{
		"accountId":"` + accountID + `",
		"to":["friend@example.com"],
		"cc":[],
		"bcc":[],
		"subject":"第一封草稿",
		"textBody":"草稿正文",
		"htmlBody":"<p>草稿正文</p>",
		"composeMode":"new",
		"forwardAttachmentIds":[],
		"localAttachmentNames":["report.pdf"]
	}`
	createResp := performRequest(router, http.MethodPost, "/api/v1/drafts", createBody, token)
	if createResp.Code != http.StatusOK {
		t.Fatalf("expected create draft 200, got %d: %s", createResp.Code, createResp.Body.String())
	}
	created := decodeEnvelope(t, createResp.Body.Bytes())["data"].(map[string]any)
	draftID := created["id"].(string)
	if draftID == "" || created["subject"] != "第一封草稿" {
		t.Fatalf("unexpected created draft: %#v", created)
	}
	names := created["localAttachmentNames"].([]any)
	if len(names) != 1 || names[0] != "report.pdf" {
		t.Fatalf("expected local attachment name to be preserved, got %#v", names)
	}

	listResp := performRequest(router, http.MethodGet, "/api/v1/drafts", "", token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected list draft 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	listData := decodeEnvelope(t, listResp.Body.Bytes())["data"].(map[string]any)
	if int(listData["total"].(float64)) != 1 {
		t.Fatalf("expected one draft, got %#v", listData)
	}

	updateBody := strings.Replace(createBody, "第一封草稿", "更新后的草稿", 1)
	updateResp := performRequest(router, http.MethodPut, "/api/v1/drafts/"+draftID, updateBody, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update draft 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}
	updated := decodeEnvelope(t, updateResp.Body.Bytes())["data"].(map[string]any)
	if updated["subject"] != "更新后的草稿" {
		t.Fatalf("expected updated subject, got %#v", updated)
	}

	detailResp := performRequest(router, http.MethodGet, "/api/v1/drafts/"+draftID, "", token)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected draft detail 200, got %d: %s", detailResp.Code, detailResp.Body.String())
	}
	if nestedString(t, decodeEnvelope(t, detailResp.Body.Bytes()), "data", "subject") != "更新后的草稿" {
		t.Fatalf("expected detail to reflect update, got %s", detailResp.Body.String())
	}

	deleteResp := performRequest(router, http.MethodDelete, "/api/v1/drafts/"+draftID, "", token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete draft 200, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
	missingResp := performRequest(router, http.MethodGet, "/api/v1/drafts/"+draftID, "", token)
	if missingResp.Code != http.StatusNotFound {
		t.Fatalf("expected deleted draft 404, got %d: %s", missingResp.Code, missingResp.Body.String())
	}
}
