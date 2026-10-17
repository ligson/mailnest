package storage

import (
	"fmt"
	"strings"
)

// 只维护派生排序列，兼容旧版本写入与重复升级，不改变原始会话日期。
func (s *Store) ensureThreadSortTime() error {
	m := s.db.gormDB.Migrator()
	if !m.HasColumn(&mailThreadModel{}, "SortAt") {
		if err := m.AddColumn(&mailThreadModel{}, "SortAt"); err != nil {
			return err
		}
	}
	var after int64
	for {
		rows, err := s.db.Query(`SELECT id FROM mail_threads WHERE id>? AND (sort_at IS NULL OR sort_at<>COALESCE(last_message_at,created_at)) ORDER BY id LIMIT 200`, after)
		if err != nil {
			return err
		}
		ids := []any{}
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
			after = id
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		if _, err = s.db.Exec(`UPDATE mail_threads SET sort_at=COALESCE(last_message_at,created_at) WHERE id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`)`, ids...); err != nil {
			return err
		}
	}
	for name, columns := range map[string]string{
		"idx_thread_sort_v2":         "user_id,sort_at DESC,id DESC",
		"idx_thread_account_sort_v2": "user_id,account_id,sort_at DESC,id DESC",
	} {
		if !m.HasIndex(&mailThreadModel{}, name) {
			if _, err := s.db.Exec(fmt.Sprintf("CREATE INDEX %s ON mail_threads (%s)", name, columns)); err != nil {
				return err
			}
		}
	}
	return nil
}
