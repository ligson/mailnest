package storage

import (
	"database/sql"
	"strings"
)

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func normalizeRetentionDays(value int) int {
	if value <= 0 {
		return 90
	}
	return value
}

func normalizeSMTPPort(value int) int {
	if value <= 0 {
		return 587
	}
	return value
}

func normalizeSentFolder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Sent"
	}
	return value
}

func normalizeRuleActionType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mark_read", "mark_unread", "star", "unstar", "mark_spam", "unmark_spam", "move_folder", "delete", "restore":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "move_folder"
	}
}

func normalizeEmailKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullTimeValue(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func nullStringValue(value sql.NullString) any {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return value.String
}

func nullInt64Value(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
