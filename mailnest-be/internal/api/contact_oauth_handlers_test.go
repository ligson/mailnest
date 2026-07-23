package api

import (
	"net/http"
	"testing"

	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
)

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
