package storage

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeDatabaseOptionsDefaultsToSQLite(t *testing.T) {
	dialect, dsn, err := normalizeDatabaseOptions(DatabaseOptions{
		Path: "/tmp/mailnest.db",
	})
	if err != nil {
		t.Fatalf("normalize database options: %v", err)
	}
	if dialect != dialectSQLite {
		t.Fatalf("expected sqlite dialect, got %s", dialect)
	}
	if !strings.HasPrefix(dsn, "/tmp/mailnest.db?") {
		t.Fatalf("expected sqlite path dsn, got %q", dsn)
	}
	if !strings.Contains(dsn, "_journal_mode=WAL") || !strings.Contains(dsn, "_busy_timeout=10000") {
		t.Fatalf("expected sqlite tuning flags in dsn, got %q", dsn)
	}
}

func TestNormalizeDatabaseOptionsSupportsAliases(t *testing.T) {
	tests := []struct {
		name    string
		options DatabaseOptions
		dialect dbDialect
	}{
		{
			name:    "mariadb alias",
			options: DatabaseOptions{Driver: "mariadb", DSN: "user:pass@tcp(localhost:3306)/mailnest?parseTime=true"},
			dialect: dialectMySQL,
		},
		{
			name:    "postgresql alias",
			options: DatabaseOptions{Driver: "postgresql", DSN: "postgres://mailnest:pass@localhost:5432/mailnest?sslmode=disable"},
			dialect: dialectPostgres,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialect, dsn, err := normalizeDatabaseOptions(tt.options)
			if err != nil {
				t.Fatalf("normalize database options: %v", err)
			}
			if dialect != tt.dialect {
				t.Fatalf("expected %s, got %s", tt.dialect, dialect)
			}
			if dsn != tt.options.DSN {
				t.Fatalf("expected dsn %q, got %q", tt.options.DSN, dsn)
			}
		})
	}
}

func TestSQLiteDatabasePathForDirectoryCreation(t *testing.T) {
	tests := []struct {
		name    string
		options DatabaseOptions
		want    string
	}{
		{
			name:    "path option",
			options: DatabaseOptions{Path: "/var/lib/mailnest/mailnest.db"},
			want:    "/var/lib/mailnest/mailnest.db",
		},
		{
			name:    "dsn fallback with query",
			options: DatabaseOptions{DSN: "/var/lib/mailnest/mailnest.db?_busy_timeout=10000"},
			want:    "/var/lib/mailnest/mailnest.db",
		},
		{
			name:    "file dsn",
			options: DatabaseOptions{Path: "file:/var/lib/mailnest/mailnest.db?cache=shared"},
			want:    "/var/lib/mailnest/mailnest.db",
		},
		{
			name:    "memory database",
			options: DatabaseOptions{Path: ":memory:"},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sqliteDatabasePath(tt.options); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestOpenExistingSQLiteAvoidsAutoMigrateTableRebuild(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mailnest.db")
	raw, err := sql.Open("sqlite3", sqliteDSN(path))
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	if _, err := raw.Exec(`
CREATE TABLE users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT,
	email TEXT,
	password_hash TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO users (email, password_hash) VALUES ('legacy@example.com', 'hash');
`); err != nil {
		_ = raw.Close()
		t.Fatalf("seed legacy sqlite: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw sqlite: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open existing sqlite store: %v", err)
	}
	defer store.Close()

	exists, err := store.sqliteColumnExists("users", "ui_theme")
	if err != nil {
		t.Fatalf("check migrated column: %v", err)
	}
	if !exists {
		t.Fatal("expected safe sqlite migration to add ui_theme")
	}
}

func TestRebindPlaceholdersForPostgres(t *testing.T) {
	query := `SELECT '?' AS literal, "?" AS identifier
FROM mail_messages
WHERE subject = ? AND body LIKE ?
-- ignored ?
AND from_addr = ?
/* ignored ? */`
	got := rebindPlaceholders(dialectPostgres, query)
	want := `SELECT '?' AS literal, "?" AS identifier
FROM mail_messages
WHERE subject = $1 AND body LIKE $2
-- ignored ?
AND from_addr = $3
/* ignored ? */`
	if got != want {
		t.Fatalf("unexpected postgres placeholders:\nwant: %s\n got: %s", want, got)
	}
	if unchanged := rebindPlaceholders(dialectMySQL, query); unchanged != query {
		t.Fatalf("mysql placeholders should remain unchanged, got %s", unchanged)
	}
}

func TestInsertIgnoreSQLByDialect(t *testing.T) {
	columns := []string{"user_id", "message_id"}
	conflict := []string{"user_id", "message_id"}
	tests := []struct {
		name    string
		dialect dbDialect
		want    string
	}{
		{
			name:    "sqlite",
			dialect: dialectSQLite,
			want:    "INSERT OR IGNORE INTO mail_message_states (user_id, message_id) VALUES (?, ?)",
		},
		{
			name:    "mysql",
			dialect: dialectMySQL,
			want:    "INSERT IGNORE INTO mail_message_states (user_id, message_id) VALUES (?, ?)",
		},
		{
			name:    "postgres",
			dialect: dialectPostgres,
			want:    "INSERT INTO mail_message_states (user_id, message_id) VALUES (?, ?) ON CONFLICT (user_id, message_id) DO NOTHING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := insertIgnoreSQL(tt.dialect, "mail_message_states", columns, conflict); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestUpsertContactSeenSQLByDialect(t *testing.T) {
	tests := []struct {
		name        string
		dialect     dbDialect
		mustContain []string
	}{
		{
			name:        "sqlite",
			dialect:     dialectSQLite,
			mustContain: []string{"ON CONFLICT(user_id, email_key) DO UPDATE", "excluded.display_name"},
		},
		{
			name:        "mysql",
			dialect:     dialectMySQL,
			mustContain: []string{"ON DUPLICATE KEY UPDATE", "VALUES(display_name)"},
		},
		{
			name:        "postgres",
			dialect:     dialectPostgres,
			mustContain: []string{"ON CONFLICT(user_id, email_key) DO UPDATE", "EXCLUDED.display_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := (&database{dialect: tt.dialect}).upsertContactSeenSQL()
			for _, part := range tt.mustContain {
				if !strings.Contains(sql, part) {
					t.Fatalf("expected %s SQL to contain %q, got %s", tt.dialect, part, sql)
				}
			}
		})
	}
}

func TestDueMailAccountsWhereByDialect(t *testing.T) {
	tests := []struct {
		name        string
		dialect     dbDialect
		mustContain string
	}{
		{name: "sqlite", dialect: dialectSQLite, mustContain: "datetime(last_sync_at, printf"},
		{name: "mysql", dialect: dialectMySQL, mustContain: "DATE_ADD(last_sync_at, INTERVAL poll_interval_minutes MINUTE)"},
		{name: "postgres", dialect: dialectPostgres, mustContain: "(poll_interval_minutes || ' minutes')::interval"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := (&database{dialect: tt.dialect}).dueMailAccountsWhere()
			if !strings.Contains(sql, tt.mustContain) {
				t.Fatalf("expected %s where clause to contain %q, got %s", tt.dialect, tt.mustContain, sql)
			}
		})
	}
}
