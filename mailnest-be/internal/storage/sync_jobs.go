package storage

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) CreateSyncJob(userID, accountID int64, triggerType, status string) (int64, error) {
	return s.db.insertAndGetID(
		`INSERT INTO mail_sync_jobs (user_id, account_id, trigger_type, status, started_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		userID,
		accountID,
		triggerType,
		status,
	)
}

func (s *Store) ListSyncJobs(query ListSyncJobsQuery) ([]MailSyncJob, int, error) {
	where := "WHERE user_id = ?"
	args := []any{query.UserID}
	if query.AccountID > 0 {
		where += " AND account_id = ?"
		args = append(args, query.AccountID)
	}
	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_sync_jobs `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 50
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT id, user_id, account_id, trigger_type, status, started_at, finished_at, new_message_count, error_message
		FROM mail_sync_jobs `+where+`
		ORDER BY started_at DESC, id DESC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]MailSyncJob, 0)
	for rows.Next() {
		var job MailSyncJob
		if err := rows.Scan(&job.ID, &job.UserID, &job.AccountID, &job.TriggerType, &job.Status, &job.StartedAt, &job.FinishedAt, &job.NewMessageCount, &job.ErrorMessage); err != nil {
			return nil, 0, err
		}
		items = append(items, job)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Store) FindSyncJobByID(userID, id int64) (MailSyncJob, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, account_id, trigger_type, status, started_at, finished_at, new_message_count, error_message
		FROM mail_sync_jobs
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	var job MailSyncJob
	if err := row.Scan(&job.ID, &job.UserID, &job.AccountID, &job.TriggerType, &job.Status, &job.StartedAt, &job.FinishedAt, &job.NewMessageCount, &job.ErrorMessage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MailSyncJob{}, ErrNotFound
		}
		return MailSyncJob{}, err
	}
	return job, nil
}

func (s *Store) FinishSyncJob(id int64, status string, newMessageCount int, errMessage string) error {
	_, err := s.db.Exec(
		`UPDATE mail_sync_jobs
		SET status = ?, finished_at = CURRENT_TIMESTAMP, new_message_count = ?, error_message = ?
		WHERE id = ?`,
		status,
		newMessageCount,
		nullIfEmpty(errMessage),
		id,
	)
	return err
}

func (s *Store) CreateSyncJobEvent(jobID int64, level, phase, message string, detailJSON string) error {
	_, err := s.db.Exec(
		`INSERT INTO mail_sync_job_events (job_id, level, phase, message, detail_json) VALUES (?, ?, ?, ?, ?)`,
		jobID,
		level,
		phase,
		message,
		nullIfEmpty(detailJSON),
	)
	return err
}

func (s *Store) ListSyncJobEvents(query ListSyncJobEventsQuery) ([]MailSyncJobEvent, int, error) {
	where := "WHERE j.user_id = ? AND e.job_id = ?"
	args := []any{query.UserID, query.JobID}
	if query.Level = strings.TrimSpace(query.Level); query.Level != "" {
		where += " AND e.level = ?"
		args = append(args, query.Level)
	}
	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_sync_job_events e JOIN mail_sync_jobs j ON j.id = e.job_id `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 100
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT e.id, e.job_id, e.level, e.phase, e.message, e.detail_json, e.created_at
		FROM mail_sync_job_events e
		JOIN mail_sync_jobs j ON j.id = e.job_id `+where+`
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]MailSyncJobEvent, 0)
	for rows.Next() {
		var event MailSyncJobEvent
		if err := rows.Scan(&event.ID, &event.JobID, &event.Level, &event.Phase, &event.Message, &event.DetailJSON, &event.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
