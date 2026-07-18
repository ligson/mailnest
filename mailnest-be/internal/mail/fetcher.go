package mail

import (
	"errors"
	"strings"
)

var ErrFolderNotFound = errors.New("IMAP 文件夹不存在")

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

type FolderInfo struct {
	Name       string
	Delimiter  string
	Attributes []string
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
	ListFolders(account AccountConfig) ([]FolderInfo, error)
	FetchFolder(account AccountConfig) ([]FetchedMessage, error)
	ListFolderUIDs(account AccountConfig) ([]string, error)
	FetchFolderByUIDs(account AccountConfig, uids []string) ([]FetchedMessage, error)
	DeleteFolderUIDs(account AccountConfig, uids []string) error
	FetchInbox(account AccountConfig) ([]FetchedMessage, error)
	ListInboxUIDs(account AccountConfig) ([]string, error)
	FetchInboxByUIDs(account AccountConfig, uids []string) ([]FetchedMessage, error)
	DeleteInboxUIDs(account AccountConfig, uids []string) error
}

type FakeFetcher struct {
	TestErr        error
	FetchErr       error
	Messages       []FetchedMessage
	FolderMessages map[string][]FetchedMessage
	FolderErrors   map[string]error
	Folders        []FolderInfo
	DeletedUIDs    []string
}

func (f *FakeFetcher) TestConnection(account AccountConfig) error {
	return f.TestErr
}

func (f *FakeFetcher) ListFolders(account AccountConfig) ([]FolderInfo, error) {
	if f.FetchErr != nil {
		return nil, f.FetchErr
	}
	if f.Folders != nil {
		return f.Folders, nil
	}
	return []FolderInfo{{Name: "INBOX"}, {Name: "Sent", Attributes: []string{"\\Sent"}}}, nil
}

func (f *FakeFetcher) FetchInbox(account AccountConfig) ([]FetchedMessage, error) {
	return f.FetchFolder(account)
}

func (f *FakeFetcher) FetchFolder(account AccountConfig) ([]FetchedMessage, error) {
	if f.FetchErr != nil {
		return nil, f.FetchErr
	}
	if f.FolderErrors != nil {
		if err, ok := f.FolderErrors[folderName(account)]; ok {
			return nil, err
		}
	}
	if f.FolderMessages != nil {
		if messages, ok := f.FolderMessages[folderName(account)]; ok {
			return messages, nil
		}
		return []FetchedMessage{}, nil
	}
	if !strings.EqualFold(folderName(account), "INBOX") {
		return []FetchedMessage{}, nil
	}
	return f.Messages, nil
}

func (f *FakeFetcher) ListInboxUIDs(account AccountConfig) ([]string, error) {
	return f.ListFolderUIDs(account)
}

func (f *FakeFetcher) ListFolderUIDs(account AccountConfig) ([]string, error) {
	if f.FetchErr != nil {
		return nil, f.FetchErr
	}
	if f.FolderErrors != nil {
		if err, ok := f.FolderErrors[folderName(account)]; ok {
			return nil, err
		}
	}
	messages, err := f.FetchFolder(account)
	if err != nil {
		return nil, err
	}
	uids := make([]string, 0, len(messages))
	for _, message := range messages {
		uids = append(uids, message.UID)
	}
	return uids, nil
}

func (f *FakeFetcher) FetchInboxByUIDs(account AccountConfig, uids []string) ([]FetchedMessage, error) {
	return f.FetchFolderByUIDs(account, uids)
}

func (f *FakeFetcher) FetchFolderByUIDs(account AccountConfig, uids []string) ([]FetchedMessage, error) {
	if f.FetchErr != nil {
		return nil, f.FetchErr
	}
	want := make(map[string]bool, len(uids))
	for _, uid := range uids {
		want[uid] = true
	}
	messages := make([]FetchedMessage, 0, len(uids))
	folderMessages, err := f.FetchFolder(account)
	if err != nil {
		return nil, err
	}
	for _, message := range folderMessages {
		if want[message.UID] {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func (f *FakeFetcher) DeleteInboxUIDs(account AccountConfig, uids []string) error {
	return f.DeleteFolderUIDs(account, uids)
}

func (f *FakeFetcher) DeleteFolderUIDs(account AccountConfig, uids []string) error {
	f.DeletedUIDs = append(f.DeletedUIDs, uids...)
	return nil
}

func IsSentFolderCandidate(folder FolderInfo) bool {
	for _, attr := range folder.Attributes {
		if strings.EqualFold(strings.TrimSpace(attr), "\\Sent") {
			return true
		}
	}
	lowerName := strings.ToLower(strings.TrimSpace(folder.Name))
	for _, marker := range []string{"sent", "已发送", "已发送邮件", "寄件", "寄件备份", "发件"} {
		if strings.Contains(lowerName, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}
