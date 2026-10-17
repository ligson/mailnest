package storage

import "testing"

func TestCacheMutationScopeConservative(t *testing.T) {
	for _, tc := range []struct {
		sql    string
		args   []any
		id     int64
		domain string
	}{
		{"UPDATE mail_messages SET subject=? WHERE user_id=? AND id=?", []any{"正文?", int64(7), 8}, 7, "mail"},
		{"DELETE FROM contacts WHERE user_id=? AND id=?", []any{7, 8}, 7, "contacts"},
		{"INSERT INTO contacts (email,user_id) VALUES (?,?)", []any{"a@example.com", 7}, 7, "contacts"},
		{"INSERT OR IGNORE INTO mail_messages (user_id,account_id) VALUES (?,?)", []any{7, 8}, 7, "mail"},
		{"INSERT INTO contacts (user_id) VALUES (?),(?)", []any{7, 8}, 0, "contacts"},
		{"UPDATE mail_messages SET subject=? WHERE user_id=? OR user_id=?", []any{"x", 7, 8}, 0, "mail"},
		{"UPDATE mail_messages SET user_id=? WHERE id=?", []any{7, 8}, 0, "mail"},
		{"UPDATE mail_messages SET subject='?' WHERE user_id=?", []any{7}, 0, "mail"},
		{"UPDATE mail_messages SET subject=? WHERE id IN (SELECT id FROM mail_messages WHERE user_id=?)", []any{"x", 7}, 0, "mail"},
		{"DELETE FROM contacts WHERE id=?", []any{8}, 0, "contacts"},
	} {
		got := queryCacheMutation(tc.sql, tc.args)
		if got.userID != tc.id || got.domain != tc.domain {
			t.Errorf("%s => %+v", tc.sql, got)
		}
	}
}

func TestCacheVersionsUserDomainAndBounds(t *testing.T) {
	var versions cacheVersions
	one, two, contact := versions.key(1, "mail"), versions.key(2, "mail"), versions.key(1, "contacts")
	versions.bump(cacheMutation{1, "mail"})
	if versions.key(1, "mail") == one || versions.key(2, "mail") != two || versions.key(1, "contacts") != contact {
		t.Fatal("跨用户或跨域失效")
	}
	versions.bump(cacheMutation{0, "mail"})
	if versions.key(2, "mail") == two || versions.key(1, "contacts") != contact {
		t.Fatal("保守失效错误")
	}
	old := versions.key(99999, "contacts")
	for i := int64(1); i < 4100; i++ {
		versions.bump(cacheMutation{i, "mail"})
	}
	if len(versions.users) > 4096 || versions.key(99999, "contacts") == old {
		t.Fatal("有界重置使旧缓存复活")
	}
}
