package storage

import (
	"context"
	"strings"
)

// latestThreadSummaries 一次查齐当前页会话的最新邮件，不读取正文或触发逐条查询。
// 每个当前页会话沿组合索引取一条最新邮件，不扫描其全部历史消息。
func (s *Store) latestThreadSummaries(ctx context.Context, userID int64, ids []int64) (map[int64]MailMessage, error) {
	result := make(map[int64]MailMessage, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	args := []any{userID}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.id, m.user_id, m.account_id, m.thread_id, m.local_folder_id, m.folder, m.imap_uid, m.subject, m.from_addr, m.to_addrs,
        m.sent_at, m.received_at, m.has_attachments, COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at, m.cc_addrs
        FROM mail_threads t
        INNER JOIN mail_messages m ON m.user_id=t.user_id AND m.id=(
            SELECT newest.id FROM mail_messages newest WHERE newest.user_id=t.user_id AND newest.thread_id=t.id
            ORDER BY newest.sort_at DESC,newest.id DESC LIMIT 1)
        LEFT JOIN mail_message_states ms ON ms.user_id=m.user_id AND ms.message_id=m.id
        WHERE t.user_id=? AND t.id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		message, err := scanMailMessageSummaryCC(rows, true)
		if err != nil {
			return nil, err
		}
		result[message.ThreadID.Int64] = message
	}
	return result, rows.Err()
}
