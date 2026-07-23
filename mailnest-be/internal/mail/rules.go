package mail

import (
	"database/sql"
	"strconv"
	"strings"

	"mailnest-be/internal/storage"
)

type RuleApplyScope string

const (
	RuleApplyScopeUnfiled  RuleApplyScope = "unfiled"
	RuleApplyScopeAll      RuleApplyScope = "all"
	RuleApplyScopeFiltered RuleApplyScope = "filtered"
)

func (s *Service) ApplyRulesToMessage(userID int64, message storage.MailMessage, overwrite bool) (bool, error) {
	rules, err := s.store.ListMailRules(userID, true)
	if err != nil {
		return false, err
	}
	attachments, _ := s.store.ListMailAttachments(userID, message.ID)
	applied := false
	for _, rule := range rules {
		if !ruleMatchesMessage(rule, message, attachments) {
			continue
		}
		changed, err := s.applyRuleAction(userID, message, rule, overwrite)
		if err != nil {
			return applied, err
		}
		applied = applied || changed
		if rule.StopOnMatch {
			break
		}
	}
	return applied, nil
}

func (s *Service) applyRuleAction(userID int64, message storage.MailMessage, rule storage.MailRule, overwrite bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(rule.ActionType)) {
	case "mark_read":
		value := true
		return true, s.store.UpsertMailMessageState(userID, message.ID, &value, nil, nil, nil)
	case "star":
		value := true
		return true, s.store.UpsertMailMessageState(userID, message.ID, nil, &value, nil, nil)
	case "mark_spam":
		value := true
		return true, s.store.UpsertMailMessageState(userID, message.ID, nil, nil, &value, nil)
	default:
		if message.LocalFolderID.Valid && !overwrite {
			return false, nil
		}
		return true, s.store.UpdateMailMessageFolder(userID, message.ID, sql.NullInt64{Int64: rule.TargetFolderID, Valid: true})
	}
}

func (s *Service) ApplyRules(userID int64, scope RuleApplyScope) (int, error) {
	query := storage.ListMailMessagesQuery{
		UserID:         userID,
		Limit:          5000,
		IncludeDeleted: true,
	}
	messages, _, err := s.store.ListMailMessagesByQuery(query)
	if err != nil {
		return 0, err
	}
	overwrite := scope == RuleApplyScopeAll
	count := 0
	for _, message := range messages {
		applied, err := s.ApplyRulesToMessage(userID, message, overwrite)
		if err != nil {
			return count, err
		}
		if applied {
			count++
		}
	}
	return count, nil
}

func (s *Service) PreviewRule(userID int64, rule storage.MailRule, limit int) (int, []storage.MailMessage, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	messages, _, err := s.store.ListMailMessagesByQuery(storage.ListMailMessagesQuery{
		UserID:         userID,
		Limit:          5000,
		IncludeDeleted: true,
	})
	if err != nil {
		return 0, nil, err
	}
	matched := 0
	samples := make([]storage.MailMessage, 0, limit)
	for _, message := range messages {
		attachments, _ := s.store.ListMailAttachments(userID, message.ID)
		if !ruleMatchesMessage(rule, message, attachments) {
			continue
		}
		matched++
		if len(samples) < limit {
			samples = append(samples, message)
		}
	}
	return matched, samples, nil
}

func ruleMatchesMessage(rule storage.MailRule, message storage.MailMessage, attachments []storage.MailAttachment) bool {
	if len(rule.Conditions) == 0 {
		return false
	}
	matchMode := strings.ToLower(strings.TrimSpace(rule.MatchMode))
	if matchMode == "any" {
		for _, condition := range rule.Conditions {
			if conditionMatchesMessage(condition, message, attachments) {
				return true
			}
		}
		return false
	}
	for _, condition := range rule.Conditions {
		if !conditionMatchesMessage(condition, message, attachments) {
			return false
		}
	}
	return true
}

func conditionMatchesMessage(condition storage.MailRuleCondition, message storage.MailMessage, attachments []storage.MailAttachment) bool {
	field := strings.ToLower(strings.TrimSpace(condition.Field))
	operator := strings.ToLower(strings.TrimSpace(condition.Operator))
	expected := strings.ToLower(strings.TrimSpace(condition.Value))

	switch field {
	case "has_attachments":
		return boolCondition(operator, message.HasAttachments)
	case "is_read":
		return boolCondition(operator, message.IsRead)
	case "starred":
		return boolCondition(operator, message.Starred)
	case "attachment_filename":
		return attachmentsContain(attachments, operator, expected, func(item storage.MailAttachment) string { return item.Filename })
	case "attachment_content_type":
		return attachmentsContain(attachments, operator, expected, func(item storage.MailAttachment) string { return nullableStringValue(item.ContentType) })
	case "attachment_size":
		return attachmentSizeCondition(attachments, operator, expected)
	}

	actual := strings.ToLower(messageFieldValue(field, message))
	switch operator {
	case "contains":
		return expected != "" && strings.Contains(actual, expected)
	case "equals":
		return actual == expected
	case "starts_with":
		return expected != "" && strings.HasPrefix(actual, expected)
	case "ends_with":
		return expected != "" && strings.HasSuffix(actual, expected)
	case "domain_equals":
		return expected != "" && strings.HasSuffix(actual, "@"+expected)
	default:
		return false
	}
}

func boolCondition(operator string, value bool) bool {
	switch operator {
	case "is_true":
		return value
	case "is_false":
		return !value
	default:
		return false
	}
}

func attachmentsContain(attachments []storage.MailAttachment, operator, expected string, pick func(storage.MailAttachment) string) bool {
	if expected == "" {
		return false
	}
	for _, attachment := range attachments {
		actual := strings.ToLower(pick(attachment))
		switch operator {
		case "contains":
			if strings.Contains(actual, expected) {
				return true
			}
		case "equals":
			if actual == expected {
				return true
			}
		case "starts_with":
			if strings.HasPrefix(actual, expected) {
				return true
			}
		case "ends_with":
			if strings.HasSuffix(actual, expected) {
				return true
			}
		}
	}
	return false
}

func attachmentSizeCondition(attachments []storage.MailAttachment, operator, expected string) bool {
	size, err := strconv.ParseInt(expected, 10, 64)
	if err != nil {
		return false
	}
	for _, attachment := range attachments {
		switch operator {
		case "equals":
			if attachment.Size == size {
				return true
			}
		case "greater_than":
			if attachment.Size > size {
				return true
			}
		case "less_than":
			if attachment.Size < size {
				return true
			}
		}
	}
	return false
}

func messageFieldValue(field string, message storage.MailMessage) string {
	switch field {
	case "from", "from_domain":
		return nullableStringValue(message.FromAddr)
	case "to":
		return nullableStringValue(message.ToAddrs)
	case "cc":
		return nullableStringValue(message.CCAddrs)
	case "subject":
		return nullableStringValue(message.Subject)
	case "body":
		return nullableStringValue(message.SearchText)
	case "account_id":
		return strconv.FormatInt(message.AccountID, 10)
	case "folder":
		return message.Folder
	default:
		return ""
	}
}

func nullableStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
