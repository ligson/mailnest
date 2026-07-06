package oauth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mailnest-be/internal/config"
)

const microsoftScope = "offline_access https://outlook.office.com/IMAP.AccessAsUser.All https://graph.microsoft.com/User.Read"

type MicrosoftHTTPExchanger struct {
	cfg config.MicrosoftOAuthConfig
}

func NewMicrosoftExchanger(cfg config.MicrosoftOAuthConfig) MicrosoftExchanger {
	return &MicrosoftHTTPExchanger{cfg: cfg}
}

func (e *MicrosoftHTTPExchanger) AuthCodeURL(state, redirectURL string) string {
	values := url.Values{}
	values.Set("client_id", e.cfg.ClientID)
	values.Set("response_type", "code")
	values.Set("redirect_uri", redirectURL)
	values.Set("response_mode", "query")
	values.Set("scope", microsoftScope)
	values.Set("state", state)
	return e.authorityURL("authorize") + "?" + values.Encode()
}

func (e *MicrosoftHTTPExchanger) Exchange(code, redirectURL string) (Token, MicrosoftAccount, error) {
	token, err := e.token(url.Values{
		"client_id":     {e.cfg.ClientID},
		"client_secret": {e.cfg.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURL},
		"scope":         {microsoftScope},
	})
	if err != nil {
		return Token{}, MicrosoftAccount{}, err
	}
	account, err := e.me(token.AccessToken)
	if err != nil {
		return Token{}, MicrosoftAccount{}, err
	}
	return token, account, nil
}

func (e *MicrosoftHTTPExchanger) Refresh(refreshToken string) (Token, error) {
	return e.token(url.Values{
		"client_id":     {e.cfg.ClientID},
		"client_secret": {e.cfg.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"scope":         {microsoftScope},
	})
}

func (e *MicrosoftHTTPExchanger) token(values url.Values) (Token, error) {
	resp, err := http.PostForm(e.authorityURL("token"), values)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Token{}, errors.New("Microsoft token endpoint returned " + resp.Status)
	}

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Token{}, err
	}
	if body.AccessToken == "" {
		return Token{}, errors.New("Microsoft token response missing access_token")
	}
	return Token{
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(body.ExpiresIn) * time.Second),
	}, nil
}

func (e *MicrosoftHTTPExchanger) me(accessToken string) (MicrosoftAccount, error) {
	req, err := http.NewRequest(http.MethodGet, "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return MicrosoftAccount{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return MicrosoftAccount{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MicrosoftAccount{}, errors.New("Microsoft Graph returned " + resp.Status)
	}
	var body struct {
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return MicrosoftAccount{}, err
	}
	email := body.Mail
	if email == "" {
		email = body.UserPrincipalName
	}
	if email == "" {
		return MicrosoftAccount{}, errors.New("Microsoft account email not found")
	}
	return MicrosoftAccount{Email: email}, nil
}

func (e *MicrosoftHTTPExchanger) authorityURL(action string) string {
	tenant := strings.TrimSpace(e.cfg.Tenant)
	if tenant == "" {
		tenant = "consumers"
	}
	return "https://login.microsoftonline.com/" + tenant + "/oauth2/v2.0/" + action
}
