package storage

import "context"

// 候选逐批到达；每批仅保留数据库排序后的前 offset+limit+1 条摘要。
// 排序与时间比较全由数据库完成，不能在 Go 中用时间转换替代 SQLite 原值顺序。
// 顺序翻页 offset=0，内存不随全部匹配邮件数增长；极深随机跳页保留兼容 SQL 路径。
func (s *Store) streamCandidateSummaryPage(ctx context.Context, q ListMailMessagesQuery, text MailSearchTextQuery, provider MailSearchPageProvider, where string, args []any, searchWhere string, searchArgs []any) ([]MailMessage, bool, error) {
	best := []MailMessage{}
	keep := q.Offset + q.Limit + 1
	available, err := provider.CandidatePages(ctx, text, func(batch []int64) error {
		// 已确认的前缀构成准确下界；更旧的新候选无需再读取正文。
		batchWhere := where
		batchArgs := append([]any{}, args...)
		if len(best) == keep {
			batchWhere += " AND (m.sort_at,m.id) > (SELECT sort_at,id FROM mail_messages WHERE user_id=? AND id=?)"
			batchArgs = append(batchArgs, q.UserID, best[len(best)-1].ID)
		}
		candidates := MailSearchCandidates{Available: true, IDs: batch}
		batchQuery := q
		batchQuery.Limit, batchQuery.Offset, batchQuery.SkipTotal = keep, 0, false
		verified, err := s.candidateSummaryPage(ctx, batchQuery, candidates, batchWhere+candidateMailWhere(candidates), batchArgs, searchWhere, searchArgs)
		if err != nil {
			return err
		}
		if len(verified) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(best)+len(batch))
		for _, message := range best {
			ids = append(ids, message.ID)
		}
		for _, message := range verified {
			ids = append(ids, message.ID)
		}
		c := MailSearchCandidates{Available: true, IDs: ids}
		params := append([]any{}, args...)
		params = append(params, keep)
		rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.user_id,m.account_id,m.thread_id,m.local_folder_id,m.folder,m.imap_uid,m.subject,m.from_addr,m.to_addrs,
 m.sent_at,m.received_at,m.has_attachments,COALESCE(ms.is_read,0),COALESCE(ms.starred,0),COALESCE(ms.is_spam,0),ms.spam_at,ms.deleted_at,m.created_at,m.updated_at
 FROM `+s.mailMessageSource(c)+` LEFT JOIN mail_message_states ms ON ms.user_id=m.user_id AND ms.message_id=m.id `+where+candidateMailWhere(c)+` ORDER BY m.sort_at DESC,m.id DESC LIMIT ?`, params...)
		if err != nil {
			return err
		}
		defer rows.Close()
		next := make([]MailMessage, 0, min(keep, len(ids)))
		for rows.Next() {
			message, err := scanMailMessageSummary(rows)
			if err != nil {
				return err
			}
			next = append(next, message)
		}
		if err = rows.Err(); err != nil {
			return err
		}
		best = next
		return nil
	})
	if err != nil || !available {
		return nil, false, err
	}
	if len(best) <= q.Offset {
		return []MailMessage{}, true, nil
	}
	return best[q.Offset:], true, nil
}
