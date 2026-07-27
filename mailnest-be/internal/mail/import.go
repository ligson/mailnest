package mail

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"mailnest-be/internal/storage"
)

type ImportEMLResult struct {
	Message  storage.MailMessage
	Inserted bool
}

func (s *Service) ImportEML(userID, accountID int64, folder string, raw []byte) (ImportEMLResult, error) {
	if _, err := s.store.FindMailAccountByID(userID, accountID); err != nil {
		return ImportEMLResult{}, err
	}
	folder = normalizeFolderName(folder)
	fetched := fetchedMessageFromRaw(raw)
	fetched.UID = importedMessageUID(fetched, raw)
	inserted, err := s.saveMessageWithTrigger(userID, accountID, folder, fetched, "import")
	if err != nil {
		return ImportEMLResult{}, err
	}
	message, err := s.store.FindMailMessageByUID(userID, accountID, folder, fetched.UID)
	if err != nil {
		return ImportEMLResult{}, err
	}
	return ImportEMLResult{Message: message, Inserted: inserted}, nil
}

func importedMessageUID(fetched FetchedMessage, raw []byte) string {
	if messageID := strings.Trim(strings.TrimSpace(fetched.MessageID), "<>"); messageID != "" {
		return "eml-" + safePath(messageID)
	}
	sum := sha256.Sum256(raw)
	return "eml-" + hex.EncodeToString(sum[:16])
}
