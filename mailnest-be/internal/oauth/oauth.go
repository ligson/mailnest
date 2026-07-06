package oauth

import "time"

type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type MicrosoftAccount struct {
	Email string
}

type MicrosoftExchanger interface {
	Exchange(code, redirectURL string) (Token, MicrosoftAccount, error)
	Refresh(refreshToken string) (Token, error)
	AuthCodeURL(state, redirectURL string) string
}

type FakeMicrosoftExchanger struct {
	Token   Token
	Account MicrosoftAccount
	Err     error
}

func (f *FakeMicrosoftExchanger) Exchange(code, redirectURL string) (Token, MicrosoftAccount, error) {
	if f.Err != nil {
		return Token{}, MicrosoftAccount{}, f.Err
	}
	return f.Token, f.Account, nil
}

func (f *FakeMicrosoftExchanger) Refresh(refreshToken string) (Token, error) {
	if f.Err != nil {
		return Token{}, f.Err
	}
	return f.Token, nil
}

func (f *FakeMicrosoftExchanger) AuthCodeURL(state, redirectURL string) string {
	return "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize?state=" + state + "&redirect_uri=" + redirectURL
}
