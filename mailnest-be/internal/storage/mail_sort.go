package storage

import (
	"fmt"
	"log"
	"strings"
)

// ensureMailSortTime 只新增派生列与索引，原始日期不改写；旧版本回退写入后再升级也会校正。
func (s *Store) ensureMailSortTime() error {
	migrator := s.db.gormDB.Migrator()
	if !migrator.HasColumn(&mailMessageModel{}, "SortAt") {
		if err := migrator.AddColumn(&mailMessageModel{}, "SortAt"); err != nil {
			return fmt.Errorf("新增邮件排序时间失败: %w", err)
		}
	}
	fixed := 0
	var afterID int64
	for {
		// 主键向前推进，避免每修复一批又从头扫描历史大表。
		rows, err := s.db.Query(`SELECT id FROM mail_messages WHERE id>? AND (sort_at IS NULL OR sort_at<>COALESCE(sent_at,received_at,created_at)) ORDER BY id LIMIT 200`, afterID)
		if err != nil {
			return err
		}
		ids := []any{}
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			ids = append(ids, id)
			afterID = id
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		_, err = s.db.Exec(`UPDATE mail_messages SET sort_at=COALESCE(sent_at,received_at,created_at) WHERE id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`)`, ids...)
		if err != nil {
			return err
		}
		fixed += len(ids)
	}
	indexes := []struct{ name, columns string }{
		{"idx_mail_list_time_v1", "user_id, sort_at DESC, id DESC"},
		{"idx_mail_folder_time_v1", "user_id, folder, sort_at DESC, id DESC"},
		{"idx_mail_account_time_v1", "user_id, account_id, sort_at DESC, id DESC"},
		{"idx_mail_local_time_v1", "user_id, local_folder_id, sort_at DESC, id DESC"},
		{"idx_mail_attachment_time_v1", "user_id, has_attachments, sort_at DESC, id DESC"},
		{"idx_mail_thread_time_v1", "user_id, thread_id, sort_at DESC, id DESC"},
		{"idx_mail_thread_folder_v1", "user_id, folder, thread_id, id"},
	}
	for _, index := range indexes {
		if !migrator.HasIndex(&mailMessageModel{}, index.name) {
			if _, err := s.db.Exec(`CREATE INDEX ` + index.name + ` ON mail_messages (` + index.columns + `)`); err != nil {
				return err
			}
		}
	}
	if fixed > 0 {
		log.Printf("邮件排序时间兼容迁移完成 rows=%d，原始日期保持不变", fixed)
	}
	return nil
}

// MySQL 的候选检索显式使用主键，防止优化器改用宽范围用户索引后再扫描大正文。
// 无索引候选时不强制主键，使普通列表仍能使用时间组合索引。
func (s *Store) mailMessageSource(c MailSearchCandidates) string {
	if s.db.dialect == dialectMySQL && c.Available && !c.Shadow && len(c.IDs) > 0 {
		return "mail_messages m FORCE INDEX (PRIMARY)"
	}
	return "mail_messages m"
}
