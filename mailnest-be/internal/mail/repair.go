package mail

import (
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
	repaired := 0
	skippedReadError := 0
	for _, message := range messages {
		checked++
		if !message.RawPath.Valid || strings.TrimSpace(message.RawPath.String) == "" {
			continue
		}
		raw, err := os.ReadFile(message.RawPath.String)
		if err != nil {
			skippedReadError++
			continue
		}
		parsed := fetchedMessageFromRaw(raw)
		currentText := readContentFile(nullableStringValue(message.TextBodyPath))
		currentHTML := readContentFile(nullableStringValue(message.HTMLBodyPath))
		if !messageNeedsParsedRepair(message, currentText, currentHTML, parsed) {
			continue
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
		}); err != nil {
			return err
		}
		repaired++
	}
	log.Printf("历史邮件解析修复完成 checked=%d repaired=%d readErrors=%d duration=%s", checked, repaired, skippedReadError, time.Since(started))
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
	return false
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
