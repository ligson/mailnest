package mail

type AccountConfig struct {
	Email       string
	Host        string
	Port        int
	TLS         bool
	Username    string
	Password    string
	AccessToken string
	AuthType    string
	Provider    string
	Folder      string
}

type FetchedMessage struct {
	UID         string
	MessageID   string
	Subject     string
	From        string
	To          []string
	CC          []string
	SentAt      string
	TextBody    string
	HTMLBody    string
	RawContent  string
	Attachments []FetchedAttachment
}

type FetchedAttachment struct {
	Filename    string
	ContentType string
	ContentID   string
	Inline      bool
	Data        []byte
}

type Fetcher interface {
	TestConnection(account AccountConfig) error
	FetchInbox(account AccountConfig) ([]FetchedMessage, error)
}

type FakeFetcher struct {
	TestErr  error
	FetchErr error
	Messages []FetchedMessage
}

func (f *FakeFetcher) TestConnection(account AccountConfig) error {
	return f.TestErr
}

func (f *FakeFetcher) FetchInbox(account AccountConfig) ([]FetchedMessage, error) {
	if f.FetchErr != nil {
		return nil, f.FetchErr
	}
	return f.Messages, nil
}
