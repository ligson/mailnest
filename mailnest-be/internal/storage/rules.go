package storage

import (
	"database/sql"
	"errors"
)

func (s *Store) CreateMailRule(params CreateMailRuleParams) (MailRule, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return MailRule{}, err
	}
	defer tx.Rollback()

	ruleID, err := tx.insertAndGetID(
		`INSERT INTO mail_rules (user_id, name, enabled, match_mode, priority, stop_on_match, action_type, target_folder_id, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.UserID,
		params.Name,
		boolToInt(params.Enabled),
		params.MatchMode,
		params.Priority,
		boolToInt(params.StopOnMatch),
		normalizeRuleActionType(params.ActionType),
		params.TargetFolderID,
		params.SortOrder,
	)
	if err != nil {
		return MailRule{}, err
	}
	for _, condition := range params.Conditions {
		if _, err := tx.Exec(
			`INSERT INTO mail_rule_conditions (rule_id, field, operator, value) VALUES (?, ?, ?, ?)`,
			ruleID,
			condition.Field,
			condition.Operator,
			condition.Value,
		); err != nil {
			return MailRule{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return MailRule{}, err
	}
	return s.FindMailRuleByID(params.UserID, ruleID)
}

func (s *Store) ListMailRules(userID int64, enabledOnly bool) ([]MailRule, error) {
	where := "WHERE user_id = ?"
	args := []any{userID}
	if enabledOnly {
		where += " AND enabled = 1"
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, name, enabled, match_mode, priority, stop_on_match, action_type, target_folder_id, sort_order, created_at, updated_at
		FROM mail_rules `+where+`
		ORDER BY priority ASC, sort_order ASC, id ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]MailRule, 0)
	for rows.Next() {
		rule, err := scanMailRule(rows)
		if err != nil {
			return nil, err
		}
		conditions, err := s.ListMailRuleConditions(rule.ID)
		if err != nil {
			return nil, err
		}
		rule.Conditions = conditions
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *Store) FindMailRuleByID(userID, id int64) (MailRule, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, name, enabled, match_mode, priority, stop_on_match, action_type, target_folder_id, sort_order, created_at, updated_at
		FROM mail_rules
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	rule, err := scanMailRule(row)
	if err != nil {
		return MailRule{}, err
	}
	conditions, err := s.ListMailRuleConditions(rule.ID)
	if err != nil {
		return MailRule{}, err
	}
	rule.Conditions = conditions
	return rule, nil
}

func (s *Store) UpdateMailRule(userID, id int64, params CreateMailRuleParams) (MailRule, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return MailRule{}, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`UPDATE mail_rules
		SET name = ?, enabled = ?, match_mode = ?, priority = ?, stop_on_match = ?, action_type = ?, target_folder_id = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		params.Name,
		boolToInt(params.Enabled),
		params.MatchMode,
		params.Priority,
		boolToInt(params.StopOnMatch),
		normalizeRuleActionType(params.ActionType),
		params.TargetFolderID,
		params.SortOrder,
		userID,
		id,
	)
	if err != nil {
		return MailRule{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return MailRule{}, err
	}
	if count == 0 {
		return MailRule{}, ErrNotFound
	}
	if _, err := tx.Exec(`DELETE FROM mail_rule_conditions WHERE rule_id = ?`, id); err != nil {
		return MailRule{}, err
	}
	for _, condition := range params.Conditions {
		if _, err := tx.Exec(
			`INSERT INTO mail_rule_conditions (rule_id, field, operator, value) VALUES (?, ?, ?, ?)`,
			id,
			condition.Field,
			condition.Operator,
			condition.Value,
		); err != nil {
			return MailRule{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return MailRule{}, err
	}
	return s.FindMailRuleByID(userID, id)
}

func (s *Store) DeleteMailRule(userID, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`DELETE FROM mail_rule_conditions WHERE rule_id IN (SELECT id FROM mail_rules WHERE user_id = ? AND id = ?)`, userID, id)
	if err != nil {
		return err
	}
	_, _ = result.RowsAffected()

	result, err = tx.Exec(`DELETE FROM mail_rules WHERE user_id = ? AND id = ?`, userID, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) ListMailRuleConditions(ruleID int64) ([]MailRuleCondition, error) {
	rows, err := s.db.Query(
		`SELECT id, rule_id, field, operator, value
		FROM mail_rule_conditions
		WHERE rule_id = ?
		ORDER BY id ASC`,
		ruleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conditions := make([]MailRuleCondition, 0)
	for rows.Next() {
		condition, err := scanMailRuleCondition(rows)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return conditions, nil
}

func scanMailRule(scanner interface {
	Scan(dest ...any) error
}) (MailRule, error) {
	var rule MailRule
	var enabled int
	var stopOnMatch int
	err := scanner.Scan(
		&rule.ID,
		&rule.UserID,
		&rule.Name,
		&enabled,
		&rule.MatchMode,
		&rule.Priority,
		&stopOnMatch,
		&rule.ActionType,
		&rule.TargetFolderID,
		&rule.SortOrder,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailRule{}, ErrNotFound
	}
	if err != nil {
		return MailRule{}, err
	}
	rule.Enabled = enabled == 1
	rule.StopOnMatch = stopOnMatch == 1
	rule.ActionType = normalizeRuleActionType(rule.ActionType)
	return rule, nil
}

func scanMailRuleCondition(scanner interface {
	Scan(dest ...any) error
}) (MailRuleCondition, error) {
	var condition MailRuleCondition
	err := scanner.Scan(
		&condition.ID,
		&condition.RuleID,
		&condition.Field,
		&condition.Operator,
		&condition.Value,
	)
	if err != nil {
		return MailRuleCondition{}, err
	}
	return condition, nil
}
