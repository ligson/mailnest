package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// 索引任务和邮件内容在同一数据库事务提交，进程重启后仍可重放。
// 不复制正文或凭据；重试时重新读取最新邮件，而不是重放过时正文。
type mailSearchTaskModel struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	UserID        int64     `gorm:"not null;index:idx_search_tasks_user"`
	MessageID     int64     `gorm:"not null"`
	Attempts      int       `gorm:"not null;default:0"`
	NextAttemptAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;index:idx_search_tasks_due"`
	CreatedAt     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (mailSearchTaskModel) TableName() string { return "mail_search_tasks" }

type MailSearchTask struct {
	ID, UserID, MessageID int64
	Attempts              int
}
type MailSearchDocument struct {
	ID, UserID                  int64
	Subject, From, To, CC, Body string
}
type MailSearchTaskStats struct{ Pending, Failed int64 }

func enqueueMailSearchTask(tx *transaction, userID, messageID int64) error {
	// 显式 UTC，不依赖连接池各连接继承的 MySQL 会话时区。
	now := time.Now().UTC()
	_, err := tx.Exec(`INSERT INTO mail_search_tasks (user_id,message_id,next_attempt_at,created_at) VALUES (?,?,?,?)`, userID, messageID, now, now)
	return err
}

func (s *Store) ListMailSearchTasks(ctx context.Context, limit int) ([]MailSearchTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,message_id,attempts FROM mail_search_tasks WHERE next_attempt_at<=? ORDER BY id LIMIT ?`, time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []MailSearchTask
	for rows.Next() {
		var task MailSearchTask
		if err := rows.Scan(&task.ID, &task.UserID, &task.MessageID, &task.Attempts); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func taskIDParams(tasks []MailSearchTask) (string, []any) {
	args := make([]any, len(tasks))
	for i, t := range tasks {
		args[i] = t.ID
	}
	return strings.TrimRight(strings.Repeat("?,", len(args)), ","), args
}
func (s *Store) CompleteMailSearchTasks(ctx context.Context, tasks []MailSearchTask) error {
	if len(tasks) == 0 {
		return nil
	}
	placeholders, args := taskIDParams(tasks)
	_, err := s.db.DB.ExecContext(ctx, rebindPlaceholders(s.db.dialect, `DELETE FROM mail_search_tasks WHERE id IN (`+placeholders+`)`), args...)
	return err
}
func (s *Store) RetryMailSearchTasks(ctx context.Context, tasks []MailSearchTask) error {
	if len(tasks) == 0 {
		return nil
	}
	placeholders, args := taskIDParams(tasks)
	maxAttempts := 0
	for _, t := range tasks {
		if t.Attempts > maxAttempts {
			maxAttempts = t.Attempts
		}
	}
	if maxAttempts > 8 {
		maxAttempts = 8
	}
	delay := time.Second * time.Duration(1<<maxAttempts)
	args = append([]any{time.Now().UTC().Add(delay)}, args...)
	_, err := s.db.DB.ExecContext(ctx, rebindPlaceholders(s.db.dialect, `UPDATE mail_search_tasks SET attempts=attempts+1,next_attempt_at=? WHERE id IN (`+placeholders+`)`), args...)
	return err
}

// 先读取待处理 ID、再查索引，避免索引写入并确认任务与查询快照交错导致漏掉新邮件。
func (s *Store) PendingMailSearchMessageIDs(ctx context.Context, userID int64, limit int) ([]int64, bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT message_id FROM mail_search_tasks WHERE user_id=? ORDER BY message_id LIMIT ?`, userID, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, false, err
		}
		ids = append(ids, id)
	}
	return ids, len(ids) > limit, rows.Err()
}
func (s *Store) MailSearchTasksStats(ctx context.Context, userID int64) (MailSearchTaskStats, error) {
	where := ""
	var args []any
	if userID > 0 {
		where = " WHERE user_id=?"
		args = append(args, userID)
	}
	var stats MailSearchTaskStats
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN attempts>0 THEN 1 ELSE 0 END),0) FROM mail_search_tasks`+where, args...).Scan(&stats.Pending, &stats.Failed)
	return stats, err
}
func (s *Store) MailSearchSourceProgress(ctx context.Context, userID, cursor, maxID int64) (int64, int64, error) {
	var done, total int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN id<=? THEN 1 ELSE 0 END),0) FROM mail_messages WHERE user_id=? AND id<=?`, cursor, userID, maxID).Scan(&total, &done)
	return done, total, err
}
func (s *Store) MaxMailSearchMessageID(ctx context.Context) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0) FROM mail_messages`).Scan(&id)
	return id, err
}

const mailSearchDocumentSelect = `SELECT id,user_id,COALESCE(subject,''),COALESCE(from_addr,''),COALESCE(to_addrs,''),COALESCE(cc_addrs,''),COALESCE(search_text,'') FROM mail_messages `

func (s *Store) ListMailSearchDocuments(ctx context.Context, after, maxID int64, limit int) ([]MailSearchDocument, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.readMailSearchDocuments(ctx, mailSearchDocumentSelect+`WHERE id>? AND id<=? ORDER BY id LIMIT ?`, after, maxID, limit)
}
func (s *Store) MailSearchDocumentsByTasks(ctx context.Context, tasks []MailSearchTask) ([]MailSearchDocument, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	if len(tasks) > 200 {
		return nil, fmt.Errorf("单批索引任务超过限制")
	}
	args := make([]any, len(tasks))
	for i, t := range tasks {
		args[i] = t.MessageID
	}
	return s.readMailSearchDocuments(ctx, mailSearchDocumentSelect+`WHERE id IN (`+strings.TrimRight(strings.Repeat("?,", len(args)), ",")+`)`, args...)
}
func (s *Store) readMailSearchDocuments(ctx context.Context, query string, args ...any) ([]MailSearchDocument, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []MailSearchDocument
	for rows.Next() {
		var d MailSearchDocument
		if err := rows.Scan(&d.ID, &d.UserID, &d.Subject, &d.From, &d.To, &d.CC, &d.Body); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}
