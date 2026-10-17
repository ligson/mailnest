package mail

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mailnest-be/internal/storage"
)

const parsedContentRepairLimit = 5000

func (s *Service) RepairStoredParsedMessages() error {
	started := time.Now()
	messages, err := s.store.ListMailMessagesWithRawContent(parsedContentRepairLimit)
	if err != nil {
		return err
	}
	checked := 0
	for _, message := range messages {
		if err := s.repairStoredParsedMessage(message); err != nil {
			return err
		}
		checked++
	}
	log.Printf("历史邮件解析修复完成 checked=%d duration=%s", checked, time.Since(started))
	return nil
}

// 每封邮件单独让行交互读取，不让启动修复长期占用后台资源。
func (s *Service) repairStoredParsedMessage(message storage.MailMessage) error {
	release, err := s.store.Workload().Background(context.Background(), "repair")
	if err != nil {
		return err
	}
	defer release()
	{
		if !message.RawPath.Valid || strings.TrimSpace(message.RawPath.String) == "" {
			return nil
		}
		raw, err := os.ReadFile(message.RawPath.String)
		if err != nil {
			return nil
		}
		parsed := fetchedMessageFromRaw(raw)
		currentText := readContentFile(nullableStringValue(message.TextBodyPath))
		currentHTML := readContentFile(nullableStringValue(message.HTMLBodyPath))
		if !messageNeedsParsedRepair(message, currentText, currentHTML, parsed) {
			return nil
		}
		textPath := nullableStringValue(message.TextBodyPath)
		htmlPath := nullableStringValue(message.HTMLBodyPath)
		messageDir := filepath.Dir(message.RawPath.String)
		if strings.TrimSpace(parsed.TextBody) != "" {
			if path, err := writeContent(messageDir, "body.txt", parsed.TextBody); err == nil {
				textPath = path
			}
		}
		if strings.TrimSpace(parsed.HTMLBody) != "" {
			if path, err := writeContent(messageDir, "body.html", parsed.HTMLBody); err == nil {
				htmlPath = path
			}
		}
		toAddrs := strings.Join(parsed.To, ", ")
		ccAddrs := strings.Join(parsed.CC, ", ")
		sentAt := sqlNullTimeForRepair(message.SentAt, parsed.SentAt)
		if err := s.store.UpdateMailMessageParsedContent(storage.UpdateMailMessageContentParams{
			UserID:       message.UserID,
			ID:           message.ID,
			MessageID:    parsed.MessageID,
			Subject:      valueOrExisting(parsed.Subject, nullableStringValue(message.Subject)),
			FromAddr:     valueOrExisting(parsed.From, nullableStringValue(message.FromAddr)),
			ToAddrs:      valueOrExisting(toAddrs, nullableStringValue(message.ToAddrs)),
			CCAddrs:      valueOrExisting(ccAddrs, nullableStringValue(message.CCAddrs)),
			TextBodyPath: textPath,
			HTMLBodyPath: htmlPath,
			SearchText:   buildSearchText(parsed, toAddrs, ccAddrs),
			InReplyTo:    parsed.InReplyTo,
			References:   parsed.References,
			SentAt:       sentAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func messageNeedsParsedRepair(message storage.MailMessage, currentText, currentHTML string, parsed FetchedMessage) bool {
	if containsEncodedWord(nullableStringValue(message.Subject)) ||
		containsEncodedWord(nullableStringValue(message.FromAddr)) ||
		containsEncodedWord(nullableStringValue(message.ToAddrs)) ||
		containsEncodedWord(nullableStringValue(message.CCAddrs)) {
		return true
	}
	if looksLikeMIMEBody(currentText) || looksLikeMIMEBody(currentHTML) {
		return true
	}
	if strings.TrimSpace(currentText) == "" && strings.TrimSpace(parsed.TextBody) != "" {
		return true
	}
	if strings.TrimSpace(currentHTML) == "" && strings.TrimSpace(parsed.HTMLBody) != "" {
		return true
	}
	if !message.SentAt.Valid && parseTime(parsed.SentAt).Valid {
		return true
	}
	return false
}

func sqlNullTimeForRepair(existing sql.NullTime, parsedValue string) sql.NullTime {
	if existing.Valid {
		return sql.NullTime{}
	}
	return parseTime(parsedValue)
}

func readContentFile(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func containsEncodedWord(value string) bool {
	return strings.Contains(strings.ToLower(value), "=?")
}

func looksLikeMIMEBody(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(trimmed, "--") &&
		(strings.Contains(lower, "content-type:") || strings.Contains(lower, "content-transfer-encoding:"))
}
