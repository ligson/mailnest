package storage

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) CreateMailRuleLog(params CreateMailRuleLogParams) (MailRuleLog, error) {
	if _, err := s.FindMailMessageByID(params.UserID, params.MessageID); err != nil {
		return MailRuleLog{}, err
	}
	logID, err := s.db.insertAndGetID(
		`INSERT INTO mail_rule_logs (
			user_id, rule_id, rule_name, message_id, matched, action_type, target_folder_id, trigger_type,
			condition_snapshot_json, result_status, result_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.UserID,
		nullablePositiveInt64(params.RuleID),
		strings.TrimSpace(params.RuleName),
		params.MessageID,
		boolToInt(params.Matched),
		strings.TrimSpace(params.ActionType),
		nullablePositiveInt64(params.TargetFolderID),
		normalizeRuleLogTrigger(params.TriggerType),
		strings.TrimSpace(params.ConditionSnapshotJSON),
		normalizeRuleLogStatus(params.ResultStatus),
		strings.TrimSpace(params.ResultMessage),
	)
	if err != nil {
		return MailRuleLog{}, err
	}
	return s.FindMailRuleLogByID(params.UserID, logID)
}

func (s *Store) FindMailRuleLogByID(userID, id int64) (MailRuleLog, error) {
	row := s.db.QueryRow(
		`SELECT l.id, l.user_id, l.rule_id, l.rule_name, l.message_id, m.subject, l.matched, l.action_type, l.target_folder_id,
			l.trigger_type, l.condition_snapshot_json, l.result_status, l.result_message, l.created_at
		FROM mail_rule_logs l
		LEFT JOIN mail_messages m ON m.user_id = l.user_id AND m.id = l.message_id
		WHERE l.user_id = ? AND l.id = ?`,
		userID,
		id,
	)
	return scanMailRuleLog(row)
}

func (s *Store) ListMailRuleLogs(query ListMailRuleLogsQuery) ([]MailRuleLog, int, error) {
	where := "WHERE l.user_id = ?"
	args := []any{query.UserID}
	if query.MessageID > 0 {
		where += " AND l.message_id = ?"
		args = append(args, query.MessageID)
	}
	if query.RuleID > 0 {
		where += " AND l.rule_id = ?"
		args = append(args, query.RuleID)
	}
	if query.ResultStatus = strings.TrimSpace(query.ResultStatus); query.ResultStatus != "" {
		where += " AND l.result_status = ?"
		args = append(args, query.ResultStatus)
	}
	if query.TriggerType = strings.TrimSpace(query.TriggerType); query.TriggerType != "" {
		where += " AND l.trigger_type = ?"
		args = append(args, query.TriggerType)
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_rule_logs l `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT l.id, l.user_id, l.rule_id, l.rule_name, l.message_id, m.subject, l.matched, l.action_type, l.target_folder_id,
			l.trigger_type, l.condition_snapshot_json, l.result_status, l.result_message, l.created_at
		FROM mail_rule_logs l
		LEFT JOIN mail_messages m ON m.user_id = l.user_id AND m.id = l.message_id `+where+`
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	logs := make([]MailRuleLog, 0)
	for rows.Next() {
		item, err := scanMailRuleLog(rows)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func scanMailRuleLog(scanner interface {
	Scan(dest ...any) error
}) (MailRuleLog, error) {
	var item MailRuleLog
	var matched int
	err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.RuleID,
		&item.RuleName,
		&item.MessageID,
		&item.MessageSubject,
		&matched,
		&item.ActionType,
		&item.TargetFolderID,
		&item.TriggerType,
		&item.ConditionSnapshotJSON,
		&item.ResultStatus,
		&item.ResultMessage,
		&item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailRuleLog{}, ErrNotFound
	}
	if err != nil {
		return MailRuleLog{}, err
	}
	item.Matched = matched == 1
	return item, nil
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func normalizeRuleLogTrigger(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "sync", "manual", "preview":
		return value
	default:
		return "manual"
	}
}

func normalizeRuleLogStatus(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "applied", "skipped", "failed":
		return value
	default:
		return "applied"
	}
}
