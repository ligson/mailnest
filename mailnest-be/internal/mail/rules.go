package mail

import (
	"database/sql"
	"strings"

	"mailnest-be/internal/storage"
)

type RuleApplyScope string

const (
	RuleApplyScopeUnfiled RuleApplyScope = "unfiled"
	RuleApplyScopeAll     RuleApplyScope = "all"
)

func (s *Service) ApplyRulesToMessage(userID int64, message storage.MailMessage, overwrite bool) (bool, error) {
	if message.LocalFolderID.Valid && !overwrite {
		return false, nil
	}
	rules, err := s.store.ListMailRules(userID, true)
	if err != nil {
		return false, err
	}
	for _, rule := range rules {
		if ruleMatchesMessage(rule, message) {
			err := s.store.UpdateMailMessageFolder(userID, message.ID, sql.NullInt64{Int64: rule.TargetFolderID, Valid: true})
			return true, err
		}
	}
	return false, nil
}

func (s *Service) ApplyRules(userID int64, scope RuleApplyScope) (int, error) {
	messages, _, err := s.store.ListMailMessagesByQuery(storage.ListMailMessagesQuery{
		UserID: userID,
		Limit:  100,
	})
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

func ruleMatchesMessage(rule storage.MailRule, message storage.MailMessage) bool {
	if len(rule.Conditions) == 0 {
		return false
	}
	matchMode := strings.ToLower(strings.TrimSpace(rule.MatchMode))
	if matchMode == "any" {
		for _, condition := range rule.Conditions {
			if conditionMatchesMessage(condition, message) {
				return true
			}
		}
		return false
	}
	for _, condition := range rule.Conditions {
		if !conditionMatchesMessage(condition, message) {
			return false
		}
	}
	return true
}

func conditionMatchesMessage(condition storage.MailRuleCondition, message storage.MailMessage) bool {
	field := strings.ToLower(strings.TrimSpace(condition.Field))
	operator := strings.ToLower(strings.TrimSpace(condition.Operator))
	expected := strings.ToLower(strings.TrimSpace(condition.Value))

	if field == "has_attachments" {
		switch operator {
		case "is_true":
			return message.HasAttachments
		case "is_false":
			return !message.HasAttachments
		default:
			return false
		}
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
