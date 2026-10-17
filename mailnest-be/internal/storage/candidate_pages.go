package storage

import (
	"context"
	"strings"
)

// candidateSummaryPage 先用轻量元数据排序，再按顺序小批复核正文。
// 找够当前页才停止，不能先截断候选集合；深页仍逐批越过所有必要结果。
func (s *Store) candidateSummaryPage(ctx context.Context, q ListMailMessagesQuery, candidates MailSearchCandidates, where string, args []any, searchWhere string, searchArgs []any) ([]MailMessage, error) {
	limit := q.Limit
	if q.SkipTotal {
		limit++
	}
	selected := []int64{}
	matched := 0
	var afterID int64
	for len(selected) < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// 每次最多读取 64 个有序元数据。先关闭连接再复核，兼容单连接 SQLite。
		batchWhere := where
		batchArgs := append([]any{}, args...)
		if afterID > 0 {
			// 使用数据库内原值比较，避免 SQLite 时间文本格式/时区转换使同时间分页重复。
			batchWhere += " AND (m.sort_at,m.id) < (SELECT sort_at,id FROM mail_messages WHERE user_id=? AND id=?)"
			batchArgs = append(batchArgs, q.UserID, afterID)
		}
		rows, err := s.db.QueryContext(ctx, `SELECT m.id FROM `+s.mailMessageSource(candidates)+` LEFT JOIN mail_message_states ms ON ms.user_id=m.user_id AND ms.message_id=m.id `+batchWhere+` ORDER BY m.sort_at DESC,m.id DESC LIMIT 64`, batchArgs...)
		if err != nil {
			return nil, err
		}
		batch := make([]int64, 0, 64)
		for rows.Next() {
			if err = rows.Scan(&afterID); err != nil {
				rows.Close()
				return nil, err
			}
			batch = append(batch, afterID)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		batchCandidates := MailSearchCandidates{Available: true, IDs: batch}
		verifyArgs := append([]any{q.UserID}, searchArgs...)
		verified, err := s.db.QueryContext(ctx, `SELECT m.id FROM `+s.mailMessageSource(batchCandidates)+` WHERE m.user_id=?`+candidateMailWhere(batchCandidates)+searchWhere, verifyArgs...)
		if err != nil {
			return nil, err
		}
		accepted := map[int64]bool{}
		for verified.Next() {
			var id int64
			if err = verified.Scan(&id); err != nil {
				verified.Close()
				return nil, err
			}
			accepted[id] = true
		}
		err = verified.Err()
		verified.Close()
		if err != nil {
			return nil, err
		}
		for _, id := range batch {
			if accepted[id] {
				if matched >= q.Offset && len(selected) < limit {
					selected = append(selected, id)
				}
				matched++
			}
		}
	}
	if len(selected) == 0 {
		return []MailMessage{}, nil
	}
	selectedCandidates := MailSearchCandidates{Available: true, IDs: selected}
	rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.user_id,m.account_id,m.thread_id,m.local_folder_id,m.folder,m.imap_uid,m.subject,m.from_addr,m.to_addrs,
 m.sent_at,m.received_at,m.has_attachments,COALESCE(ms.is_read,0),COALESCE(ms.starred,0),COALESCE(ms.is_spam,0),ms.spam_at,ms.deleted_at,m.created_at,m.updated_at
 FROM `+s.mailMessageSource(selectedCandidates)+` LEFT JOIN mail_message_states ms ON ms.user_id=m.user_id AND ms.message_id=m.id WHERE m.user_id=?`+candidateMailWhere(selectedCandidates)+` ORDER BY m.sort_at DESC,m.id DESC`, q.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MailMessage{}
	for rows.Next() {
		item, err := scanMailMessageSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func withoutSearchClause(where string, args []any, searchWhere string, start, count int) (string, []any) {
	metadataArgs := append([]any{}, args[:start]...)
	metadataArgs = append(metadataArgs, args[start+count:]...)
	return strings.Replace(where, searchWhere, "", 1), metadataArgs
}
