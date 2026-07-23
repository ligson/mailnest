package mail

import (
	"sync"

	"mailnest-be/internal/oauth"
	"mailnest-be/internal/storage"
)

type Service struct {
	store            *storage.Store
	fetcher          Fetcher
	sender           Sender
	exchanger        oauth.MicrosoftExchanger
	dataDir          string
	credentialSecret string
	fullSyncMu       sync.Mutex
	fullSyncCancels  map[string]chan struct{}
	inboxSyncMu      sync.Mutex
	inboxSyncs       map[string]struct{}
}

func NewService(store *storage.Store, fetcher Fetcher, exchanger oauth.MicrosoftExchanger, dataDir, credentialSecret string) *Service {
	if fetcher == nil {
		fetcher = NewIMAPFetcher()
	}
	return NewServiceWithSender(store, fetcher, &SMTPSender{}, exchanger, dataDir, credentialSecret)
}

func NewServiceWithSender(store *storage.Store, fetcher Fetcher, sender Sender, exchanger oauth.MicrosoftExchanger, dataDir, credentialSecret string) *Service {
	if fetcher == nil {
		fetcher = NewIMAPFetcher()
	}
	if sender == nil {
		sender = &SMTPSender{}
	}
	return &Service{
		store:            store,
		fetcher:          fetcher,
		sender:           sender,
		exchanger:        exchanger,
		dataDir:          dataDir,
		credentialSecret: credentialSecret,
		fullSyncCancels:  make(map[string]chan struct{}),
		inboxSyncs:       make(map[string]struct{}),
	}
}
