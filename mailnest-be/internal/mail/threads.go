package mail

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"

	"mailnest-be/internal/storage"
)

var threadPrefixPattern = regexp.MustCompile(`(?i)^\s*(re|fw|fwd|回复|答复|转发)\s*[:：]\s*`)
var messageIDPattern = regexp.MustCompile(`<[^<>]+>`)

func (s *Service) ResolveThreadForMessage(userID int64, message storage.MailMessage) (storage.MailThread, error) {
	if message.ThreadID.Valid {
		thread, err := s.store.FindMailThreadByID(userID, message.ThreadID.Int64)
		if err == nil {
			return thread, nil
		}
	}
	if message.SourceMessageID.Valid {
		thread, err := s.store.FindMailThreadBySourceMessageID(userID, message.SourceMessageID.Int64)
		if err == nil {
			return s.attachMessageToThread(userID, message, thread.ID)
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return storage.MailThread{}, err
		}
	}
	references := threadReferenceCandidates(message)
	if len(references) > 0 {
		thread, err := s.store.FindMailThreadByReferencedMessageIDs(userID, references)
		if err == nil {
			return s.attachMessageToThread(userID, message, thread.ID)
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return storage.MailThread{}, err
		}
	}
	normalizedSubject := NormalizeThreadSubject(nullableStringValue(message.Subject))
	if normalizedSubject != "" {
		thread, err := s.store.FindMailThreadByNormalizedSubject(userID, message.AccountID, normalizedSubject)
		if err == nil {
			return s.attachMessageToThread(userID, message, thread.ID)
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return storage.MailThread{}, err
		}
	}

	thread, err := s.store.CreateMailThread(storage.CreateMailThreadParams{
		UserID:            userID,
		AccountID:         message.AccountID,
		RootMessageID:     sql.NullInt64{Int64: message.ID, Valid: true},
		Subject:           nullableStringValue(message.Subject),
		NormalizedSubject: normalizedSubject,
		LastMessageAt:     messageTime(message),
		HasAttachments:    message.HasAttachments,
	})
	if err != nil {
		return storage.MailThread{}, err
	}
	return s.attachMessageToThread(userID, message, thread.ID)
}

func (s *Service) RebuildThreads(userID, accountID int64, scope string) (storage.RebuildThreadsResult, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "all" {
		if err := s.store.ResetMailThreads(userID, accountID); err != nil {
			return storage.RebuildThreadsResult{}, err
		}
	}
	messages, err := s.store.ListMailMessagesForThreadRebuild(storage.RebuildThreadsParams{
		UserID:    userID,
		AccountID: accountID,
		Scope:     scope,
	})
	if err != nil {
		return storage.RebuildThreadsResult{}, err
	}
	for _, message := range messages {
		if _, err := s.ResolveThreadForMessage(userID, message); err != nil {
			return storage.RebuildThreadsResult{}, err
		}
	}
	threadCount, err := s.store.CountMailThreads(userID)
	if err != nil {
		return storage.RebuildThreadsResult{}, err
	}
	return storage.RebuildThreadsResult{ProcessedCount: len(messages), ThreadCount: threadCount}, nil
}

func (s *Service) attachMessageToThread(userID int64, message storage.MailMessage, threadID int64) (storage.MailThread, error) {
	if !message.ThreadID.Valid || message.ThreadID.Int64 != threadID {
		if err := s.store.SetMailMessageThread(userID, message.ID, threadID); err != nil {
			return storage.MailThread{}, err
		}
	}
	if err := s.store.RefreshMailThreadStats(userID, threadID); err != nil {
		return storage.MailThread{}, err
	}
	return s.store.FindMailThreadByID(userID, threadID)
}

func NormalizeThreadSubject(subject string) string {
	subject = strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(subject))), " ")
	for {
		next := threadPrefixPattern.ReplaceAllString(subject, "")
		next = strings.TrimSpace(next)
		if next == subject {
			break
		}
		subject = next
	}
	return strings.Join(strings.Fields(subject), " ")
}

func threadReferenceCandidates(message storage.MailMessage) []string {
	values := make([]string, 0)
	values = append(values, messageIDTokens(nullableStringValue(message.References))...)
	values = append(values, messageIDTokens(nullableStringValue(message.InReplyTo))...)
	if self := strings.TrimSpace(nullableStringValue(message.MessageID)); self != "" {
		values = append(values, self)
	}
	return compactThreadStrings(values)
}

func messageIDTokens(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	matches := messageIDPattern.FindAllString(value, -1)
	if len(matches) > 0 {
		return matches
	}
	return strings.Fields(value)
}

func compactThreadStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func messageTime(message storage.MailMessage) sql.NullTime {
	if message.SentAt.Valid {
		return message.SentAt
	}
	if message.ReceivedAt.Valid {
		return message.ReceivedAt
	}
	return sql.NullTime{Time: message.CreatedAt, Valid: !message.CreatedAt.IsZero()}
}
