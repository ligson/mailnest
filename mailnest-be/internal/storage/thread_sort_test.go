package storage

import (
	"database/sql"
	"testing"
	"time"
)

func TestThreadSortMigrationIdempotentAndOriginalDate(t *testing.T) {
	s := performanceStore(t)
	userID := checkMailSearchResults(t, s)
	thread, err := s.CreateMailThread(CreateMailThreadParams{UserID: userID, Subject: "迁移不改历史日期"})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	if _, err = s.db.Exec("UPDATE mail_threads SET last_message_at=?,sort_at=NULL WHERE user_id=? AND id=?", at, userID, thread.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.ensureThreadSortTime(); err != nil {
			t.Fatal(err)
		}
	}
	var sorted, last sql.NullTime
	if err = s.db.QueryRow("SELECT sort_at,last_message_at FROM mail_threads WHERE user_id=? AND id=?", userID, thread.ID).Scan(&sorted, &last); err != nil {
		t.Fatal(err)
	}
	if !sorted.Time.Equal(at) || !last.Time.Equal(at) {
		t.Fatalf("历史时间被修改: %v/%v", sorted, last)
	}
	for _, index := range []string{"idx_thread_sort_v2", "idx_thread_account_sort_v2"} {
		if !s.db.gormDB.Migrator().HasIndex(&mailThreadModel{}, index) {
			t.Fatal("缺少索引", index)
		}
	}
}
