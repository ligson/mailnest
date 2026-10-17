package storage

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

func (s *Store) MailReadVersion() string {
	return fmt.Sprintf("%d-%d", s.db.instance, s.db.mailRevision.Load())
}

func (s *Store) MailReadVersionForUser(userID int64) string {
	return fmt.Sprintf("%d-%s", s.db.instance, s.db.cacheVersions.key(userID, "mail"))
}

// 池等待差值为采样区间全局累计值，不冒充单个请求的精确连接等待时间。
func (s *Store) observeMailPhase(userID int64, phase string, start time.Time, before sql.DBStats, candidates int) {
	duration := time.Since(start)
	if duration < 250*time.Millisecond {
		return
	}
	after := s.db.Stats()
	log.Printf("邮件查询阶段较慢 user_id=%d phase=%s duration=%s engine=%s candidates=%d pool_in_use=%d pool_wait_count_delta=%d pool_wait_duration_delta=%s", userID, phase, duration, s.MailSearchMode(), candidates, after.InUse, after.WaitCount-before.WaitCount, after.WaitDuration-before.WaitDuration)
}
