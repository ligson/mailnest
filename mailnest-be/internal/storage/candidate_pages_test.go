package storage

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type fixedReadCandidates struct{ ids []int64 }

func (f fixedReadCandidates) Candidates(context.Context, MailSearchTextQuery) (MailSearchCandidates, error) {
	return MailSearchCandidates{IDs: f.ids, Available: true}, nil
}
func (fixedReadCandidates) ObserveShadow(int64, []int64, MailSearchCandidates) {}
func (fixedReadCandidates) Mode() string                                       { return "active" }

func TestCandidateSummaryPageNeverTruncatesSparseMatches(t *testing.T) {
	s := performanceStore(t)
	userID := checkMailSearchResults(t, s)
	accounts, err := s.ListMailAccounts(userID)
	if err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	for i := 0; i < 150; i++ {
		subject := "不匹配"
		if i%17 == 0 {
			subject = "稀疏命中"
		}
		item, _, err := s.InsertMailMessageIfNew(CreateMailMessageParams{UserID: userID, AccountID: accounts[0].ID, Folder: "INBOX", IMAPUID: fmt.Sprintf("sparse-%d", i), Subject: subject})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, item.ID)
	}
	for _, offset := range []int{0, 3, 6, 9, 100} {
		q := ListMailMessagesQuery{UserID: userID, Subject: "稀疏命中", SystemFolder: "inbox", SummaryOnly: true, Limit: 3, Offset: offset, SkipTotal: true}
		s.SetMailSearchProvider(nil)
		expected, _, err := s.ListMailMessagesByQuery(q)
		if err != nil {
			t.Fatal(err)
		}
		s.SetMailSearchProvider(fixedReadCandidates{ids})
		actual, _, err := s.ListMailMessagesByQuery(q)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("稀疏分页漏信 offset=%d got=%d want=%d", offset, len(actual), len(expected))
		}
	}
}
