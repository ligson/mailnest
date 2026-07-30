package mail

import (
	"fmt"
	"log"
	"strings"
	"time"
)

type RepairMissingSentAtResult struct {
	Checked       int
	Repaired      int
	NoValidDate   int
	ReadErrors    int
	DatabaseSkips int
}

// RepairMissingSentAt 只读取原始 EML 头部的 Date，补齐当前为空的 sent_at。
// 该方法不会重解析正文、附件或覆盖已有时间，适用于大批量导入后的受控修复。
func (s *Service) RepairMissingSentAt(userID, accountID int64, importedOnly bool, batchSize int) (RepairMissingSentAtResult, error) {
	if userID <= 0 || accountID <= 0 {
		return RepairMissingSentAtResult{}, fmt.Errorf("用户 ID 和邮箱账号 ID 必须大于 0")
	}
	if batchSize <= 0 || batchSize > 1000 {
		batchSize = 500
	}
	started := time.Now()
	result := RepairMissingSentAtResult{}
	beforeID := int64(1<<62 - 1)

	for {
		messages, err := s.store.ListMailMessagesMissingSentAt(userID, accountID, beforeID, importedOnly, batchSize)
		if err != nil {
			return result, err
		}
		if len(messages) == 0 {
			break
		}
		for _, message := range messages {
			beforeID = message.ID
			result.Checked++
			if !message.RawPath.Valid || strings.TrimSpace(message.RawPath.String) == "" {
				result.ReadErrors++
				continue
			}
			sentAt, err := sentAtFromRawHeaderFile(message.RawPath.String)
			if err != nil {
				result.ReadErrors++
				log.Printf("修复邮件发送时间时读取原文头失败 userID=%d accountID=%d messageID=%d err=%v", userID, accountID, message.ID, err)
				continue
			}
			if !sentAt.Valid {
				result.NoValidDate++
				continue
			}
			updated, err := s.store.UpdateMailMessageSentAtIfMissing(userID, message.ID, sentAt.Time)
			if err != nil {
				return result, err
			}
			if updated {
				result.Repaired++
			} else {
				result.DatabaseSkips++
			}
		}
	}
	log.Printf("缺失邮件发送时间修复完成 userID=%d accountID=%d importedOnly=%t checked=%d repaired=%d noValidDate=%d readErrors=%d databaseSkips=%d duration=%s",
		userID, accountID, importedOnly, result.Checked, result.Repaired, result.NoValidDate, result.ReadErrors, result.DatabaseSkips, time.Since(started))
	return result, nil
}
