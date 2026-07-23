package mail

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	netmail "net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mailnest-be/internal/storage"
)

// saveMessage 是邮件入库的统一入口：先把原文/正文/附件落盘，再写入数据库元数据。
func (s *Service) saveMessage(userID, accountID int64, folder string, fetched FetchedMessage) (bool, error) {
	folder = normalizeFolderName(folder)
	uid := strings.TrimSpace(fetched.UID)
	if uid == "" {
		uid = strings.TrimSpace(fetched.MessageID)
	}
	if uid == "" {
		uid = fmt.Sprintf("generated-%d", time.Now().UnixNano())
	}

	messageDir := filepath.Join(s.dataDir, "users", fmt.Sprint(userID), "accounts", fmt.Sprint(accountID), "messages", safePath(folder), safePath(uid))
	if err := os.MkdirAll(messageDir, 0o755); err != nil {
		return false, err
	}

	rawPath, err := writeContent(messageDir, "raw.eml", fetched.RawContent)
	if err != nil {
		return false, err
	}
	textPath, err := writeContent(messageDir, "body.txt", fetched.TextBody)
	if err != nil {
		return false, err
	}
	htmlPath, err := writeContent(messageDir, "body.html", fetched.HTMLBody)
	if err != nil {
		return false, err
	}

	sentAt := parseTime(fetched.SentAt)
	receivedAt := sql.NullTime{Time: time.Now(), Valid: true}
	toAddrs := strings.Join(fetched.To, ", ")
	ccAddrs := strings.Join(fetched.CC, ", ")

	_, inserted, err := s.store.InsertMailMessageIfNew(storage.CreateMailMessageParams{
		UserID:          userID,
		AccountID:       accountID,
		Folder:          folder,
		IMAPUID:         uid,
		MessageID:       fetched.MessageID,
		Subject:         fetched.Subject,
		FromAddr:        fetched.From,
		ToAddrs:         toAddrs,
		CCAddrs:         ccAddrs,
		SentAt:          sentAt,
		ReceivedAt:      receivedAt,
		HasAttachments:  len(fetched.Attachments) > 0,
		TextBodyPath:    textPath,
		HTMLBodyPath:    htmlPath,
		RawPath:         rawPath,
		SearchText:      buildSearchText(fetched, toAddrs, ccAddrs),
		InReplyTo:       fetched.InReplyTo,
		References:      fetched.References,
		SourceMessageID: sql.NullInt64{Int64: fetched.SourceMessageID, Valid: fetched.SourceMessageID > 0},
		ComposeMode:     fetched.ComposeMode,
	})
	if err != nil {
		return false, err
	}
	if err := s.upsertContactsFromFetchedMessage(userID, fetched, receivedAt); err != nil {
		log.Printf("邮件联系人沉淀失败 userID=%d accountID=%d folder=%s uid=%s err=%v", userID, accountID, folder, uid, err)
	}
	if !inserted {
		if len(fetched.Attachments) > 0 {
			message, err := s.store.FindMailMessageByUID(userID, accountID, folder, uid)
			if err != nil {
				return false, err
			}
			existingAttachments, err := s.store.ListMailAttachments(userID, message.ID)
			if err != nil {
				return false, err
			}
			if len(existingAttachments) == 0 {
				log.Printf("邮件重复命中但附件缺失，开始回填附件 userID=%d accountID=%d messageID=%d folder=%s uid=%s attachmentCount=%d", userID, accountID, message.ID, folder, uid, len(fetched.Attachments))
				for index, attachment := range fetched.Attachments {
					if err := s.saveAttachment(userID, message.ID, messageDir, index, attachment); err != nil {
						return false, err
					}
				}
				if err := s.store.UpdateMailMessageHasAttachments(userID, message.ID, true); err != nil {
					return false, err
				}
			}
		}
		return false, nil
	}
	message, err := s.store.FindMailMessageByUID(userID, accountID, folder, uid)
	if err != nil {
		return false, err
	}
	if len(fetched.Attachments) > 0 {
		log.Printf("保存新邮件附件 userID=%d accountID=%d messageID=%d folder=%s uid=%s attachmentCount=%d", userID, accountID, message.ID, folder, uid, len(fetched.Attachments))
	}
	for index, attachment := range fetched.Attachments {
		if err := s.saveAttachment(userID, message.ID, messageDir, index, attachment); err != nil {
			return false, err
		}
	}
	thread, err := s.ResolveThreadForMessage(userID, message)
	if err != nil {
		return false, err
	}
	message.ThreadID = sql.NullInt64{Int64: thread.ID, Valid: true}
	if _, err := s.ApplyRulesToMessageWithTrigger(userID, message, false, "sync"); err != nil {
		return false, err
	}
	return inserted, nil
}

func (s *Service) upsertContactsFromFetchedMessage(userID int64, fetched FetchedMessage, seenAt sql.NullTime) error {
	for _, candidate := range contactCandidatesFromFetchedMessage(fetched) {
		if _, err := s.store.UpsertContactSeen(storage.CreateContactParams{
			UserID:      userID,
			Email:       candidate.email,
			DisplayName: candidate.name,
			Source:      "auto",
			SeenAt:      seenAt,
		}); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return err
		}
	}
	return nil
}

func (s *Service) upsertBCCContacts(userID int64, values []string, seenAt time.Time) error {
	for _, value := range values {
		for _, candidate := range parseContactCandidates(value) {
			if _, err := s.store.UpsertContactSeen(storage.CreateContactParams{
				UserID:      userID,
				Email:       candidate.email,
				DisplayName: candidate.name,
				Source:      "auto",
				SeenAt:      sql.NullTime{Time: seenAt, Valid: true},
			}); err != nil && !errors.Is(err, storage.ErrNotFound) {
				return err
			}
		}
	}
	return nil
}

type contactCandidate struct {
	email string
	name  string
}

func contactCandidatesFromFetchedMessage(fetched FetchedMessage) []contactCandidate {
	values := []string{fetched.From}
	values = append(values, fetched.To...)
	values = append(values, fetched.CC...)
	seen := make(map[string]bool)
	candidates := make([]contactCandidate, 0, len(values))
	for _, value := range values {
		for _, candidate := range parseContactCandidates(value) {
			key := strings.ToLower(candidate.email)
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func parseContactCandidates(value string) []contactCandidate {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	addresses, err := netmail.ParseAddressList(value)
	if err != nil {
		address, singleErr := netmail.ParseAddress(value)
		if singleErr != nil {
			return nil
		}
		addresses = []*netmail.Address{address}
	}
	candidates := make([]contactCandidate, 0, len(addresses))
	for _, address := range addresses {
		email := strings.ToLower(strings.TrimSpace(address.Address))
		if email == "" {
			continue
		}
		candidates = append(candidates, contactCandidate{
			email: email,
			name:  strings.TrimSpace(address.Name),
		})
	}
	return candidates
}

func writeContent(dir, name, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) saveAttachment(userID, messageID int64, messageDir string, index int, attachment FetchedAttachment) error {
	if len(attachment.Data) == 0 {
		return nil
	}
	attachmentDir := filepath.Join(messageDir, "attachments")
	if err := os.MkdirAll(attachmentDir, 0o755); err != nil {
		return err
	}
	filename := strings.TrimSpace(attachment.Filename)
	if filename == "" {
		filename = fmt.Sprintf("attachment-%d", index+1)
	}
	filePath := filepath.Join(attachmentDir, fmt.Sprintf("%03d-%s", index+1, safePath(filename)))
	if err := os.WriteFile(filePath, attachment.Data, 0o600); err != nil {
		return err
	}
	_, err := s.store.CreateMailAttachment(storage.CreateMailAttachmentParams{
		UserID:      userID,
		MessageID:   messageID,
		Filename:    filename,
		ContentType: attachment.ContentType,
		ContentID:   attachment.ContentID,
		Inline:      attachment.Inline,
		Size:        int64(len(attachment.Data)),
		FilePath:    filePath,
	})
	return err
}
