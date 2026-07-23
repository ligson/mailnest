package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DatabaseOptions struct {
	Driver       string
	DSN          string
	Path         string
	MaxOpenConns int
	MaxIdleConns int
}

type dbDialect string

const (
	dialectSQLite   dbDialect = "sqlite"
	dialectMySQL    dbDialect = "mysql"
	dialectPostgres dbDialect = "postgres"
)

type database struct {
	*sql.DB
	gormDB  *gorm.DB
	dialect dbDialect
}

type transaction struct {
	*sql.Tx
	dialect dbDialect
}

func openDatabase(options DatabaseOptions) (*database, error) {
	dialect, dsn, err := normalizeDatabaseOptions(options)
	if err != nil {
		return nil, err
	}
	if dialect == dialectSQLite {
		path := sqliteDatabasePath(options)
		if path != "" {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
		}
	}

	gormDB, err := gorm.Open(gormDialector(dialect, dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	raw, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	maxOpenConns := options.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = defaultMaxOpenConns(dialect)
	}
	maxIdleConns := options.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = maxOpenConns
	}
	raw.SetMaxOpenConns(maxOpenConns)
	raw.SetMaxIdleConns(maxIdleConns)

	db := &database{DB: raw, gormDB: gormDB, dialect: dialect}
	if err := db.configure(); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := raw.Ping(); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return db, nil
}

func sqliteDatabasePath(options DatabaseOptions) string {
	path := strings.TrimSpace(options.Path)
	if path == "" {
		path = strings.TrimSpace(options.DSN)
	}
	if path == "" || path == ":memory:" || strings.HasPrefix(path, "file::memory:") {
		return ""
	}
	if index := strings.Index(path, "?"); index >= 0 {
		path = path[:index]
	}
	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
	}
	return strings.TrimSpace(path)
}

func normalizeDatabaseOptions(options DatabaseOptions) (dbDialect, string, error) {
	driver := strings.ToLower(strings.TrimSpace(options.Driver))
	if driver == "" {
		driver = "sqlite"
	}
	switch driver {
	case "sqlite", "sqlite3":
		path := strings.TrimSpace(options.Path)
		if path == "" {
			path = strings.TrimSpace(options.DSN)
		}
		if path == "" {
			return "", "", fmt.Errorf("sqlite database path is required")
		}
		return dialectSQLite, sqliteDSN(path), nil
	case "mysql", "mariadb":
		dsn := strings.TrimSpace(options.DSN)
		if dsn == "" {
			return "", "", fmt.Errorf("mysql database dsn is required")
		}
		return dialectMySQL, dsn, nil
	case "postgres", "postgresql":
		dsn := strings.TrimSpace(options.DSN)
		if dsn == "" {
			return "", "", fmt.Errorf("postgres database dsn is required")
		}
		return dialectPostgres, dsn, nil
	default:
		return "", "", fmt.Errorf("unsupported database driver %q", options.Driver)
	}
}

func gormDialector(dialect dbDialect, dsn string) gorm.Dialector {
	switch dialect {
	case dialectMySQL:
		return mysql.New(mysql.Config{
			DSN:                      dsn,
			DisableDatetimePrecision: true,
		})
	case dialectPostgres:
		return postgres.Open(dsn)
	default:
		return sqlite.Open(dsn)
	}
}

func defaultMaxOpenConns(dialect dbDialect) int {
	if dialect == dialectSQLite {
		return 4
	}
	return 16
}

func (db *database) configure() error {
	switch db.dialect {
	case dialectSQLite:
		_, err := db.DB.Exec(`PRAGMA journal_mode = WAL; PRAGMA synchronous = NORMAL; PRAGMA busy_timeout = 10000; PRAGMA temp_store = MEMORY;`)
		return err
	case dialectMySQL:
		_, err := db.DB.Exec(`SET time_zone = '+00:00'`)
		return err
	default:
		return nil
	}
}

func (db *database) Exec(query string, args ...any) (sql.Result, error) {
	return db.DB.Exec(rebindPlaceholders(db.dialect, query), args...)
}

func (db *database) Query(query string, args ...any) (*sql.Rows, error) {
	return db.DB.Query(rebindPlaceholders(db.dialect, query), args...)
}

func (db *database) QueryRow(query string, args ...any) *sql.Row {
	return db.DB.QueryRow(rebindPlaceholders(db.dialect, query), args...)
}

func (db *database) Begin() (*transaction, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &transaction{Tx: tx, dialect: db.dialect}, nil
}

func (db *database) insertAndGetID(query string, args ...any) (int64, error) {
	if db.dialect == dialectPostgres {
		var id int64
		err := db.QueryRow(query+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	result, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *database) insertIgnoreSQL(table string, columns []string, conflictColumns []string) string {
	return insertIgnoreSQL(db.dialect, table, columns, conflictColumns)
}

func (tx *transaction) insertIgnoreSQL(table string, columns []string, conflictColumns []string) string {
	return insertIgnoreSQL(tx.dialect, table, columns, conflictColumns)
}

func insertIgnoreSQL(dialect dbDialect, table string, columns []string, conflictColumns []string) string {
	columnList := strings.Join(columns, ", ")
	placeholders := strings.TrimRight(strings.Repeat("?, ", len(columns)), ", ")
	switch dialect {
	case dialectMySQL:
		return fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", table, columnList, placeholders)
	case dialectPostgres:
		conflictList := strings.Join(conflictColumns, ", ")
		return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO NOTHING", table, columnList, placeholders, conflictList)
	default:
		return fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", table, columnList, placeholders)
	}
}

func (db *database) upsertContactSeenSQL() string {
	switch db.dialect {
	case dialectMySQL:
		return `INSERT INTO contacts (
			user_id, email, email_key, display_name, source, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			email = VALUES(email),
			display_name = CASE
				WHEN contacts.display_name IS NULL OR TRIM(contacts.display_name) = '' THEN VALUES(display_name)
				ELSE contacts.display_name
			END,
			source = CASE
				WHEN contacts.source = 'auto' THEN VALUES(source)
				ELSE contacts.source
			END,
			last_seen_at = VALUES(last_seen_at),
			updated_at = CURRENT_TIMESTAMP`
	case dialectPostgres:
		return `INSERT INTO contacts (
			user_id, email, email_key, display_name, source, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, email_key) DO UPDATE SET
			email = EXCLUDED.email,
			display_name = CASE
				WHEN contacts.display_name IS NULL OR TRIM(contacts.display_name) = '' THEN EXCLUDED.display_name
				ELSE contacts.display_name
			END,
			source = CASE
				WHEN contacts.source = 'auto' THEN EXCLUDED.source
				ELSE contacts.source
			END,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = CURRENT_TIMESTAMP`
	default:
		return `INSERT INTO contacts (
			user_id, email, email_key, display_name, source, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, email_key) DO UPDATE SET
			email = excluded.email,
			display_name = CASE
				WHEN contacts.display_name IS NULL OR TRIM(contacts.display_name) = '' THEN excluded.display_name
				ELSE contacts.display_name
			END,
			source = CASE
				WHEN contacts.source = 'auto' THEN excluded.source
				ELSE contacts.source
			END,
			last_seen_at = excluded.last_seen_at,
			updated_at = CURRENT_TIMESTAMP`
	}
}

func (db *database) dueMailAccountsWhere() string {
	switch db.dialect {
	case dialectMySQL:
		return `WHERE enabled = 1
			AND poll_interval_minutes > 0
			AND COALESCE(full_sync_status, 'idle') != 'running'
			AND (
				last_sync_at IS NULL
				OR DATE_ADD(last_sync_at, INTERVAL poll_interval_minutes MINUTE) <= UTC_TIMESTAMP()
			)`
	case dialectPostgres:
		return `WHERE enabled = 1
			AND poll_interval_minutes > 0
			AND COALESCE(full_sync_status, 'idle') != 'running'
			AND (
				last_sync_at IS NULL
				OR last_sync_at + (poll_interval_minutes || ' minutes')::interval <= NOW()
			)`
	default:
		return `WHERE enabled = 1
			AND poll_interval_minutes > 0
			AND COALESCE(full_sync_status, 'idle') != 'running'
			AND (
				last_sync_at IS NULL
				OR datetime(last_sync_at, printf('+%d minutes', poll_interval_minutes)) <= CURRENT_TIMESTAMP
			)`
	}
}

func (tx *transaction) Exec(query string, args ...any) (sql.Result, error) {
	return tx.Tx.Exec(rebindPlaceholders(tx.dialect, query), args...)
}

func (tx *transaction) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.Tx.Query(rebindPlaceholders(tx.dialect, query), args...)
}

func (tx *transaction) QueryRow(query string, args ...any) *sql.Row {
	return tx.Tx.QueryRow(rebindPlaceholders(tx.dialect, query), args...)
}

func (tx *transaction) insertAndGetID(query string, args ...any) (int64, error) {
	if tx.dialect == dialectPostgres {
		var id int64
		err := tx.QueryRow(query+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func rebindPlaceholders(dialect dbDialect, query string) string {
	if dialect != dialectPostgres || !strings.Contains(query, "?") {
		return query
	}
	var builder strings.Builder
	builder.Grow(len(query) + 8)
	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false
	argIndex := 1
	for i := 0; i < len(query); i++ {
		ch := query[i]
		next := byte(0)
		if i+1 < len(query) {
			next = query[i+1]
		}
		switch {
		case inLineComment:
			builder.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
		case inBlockComment:
			builder.WriteByte(ch)
			if ch == '*' && next == '/' {
				i++
				builder.WriteByte('/')
				inBlockComment = false
			}
		case inSingleQuote:
			builder.WriteByte(ch)
			if ch == '\'' {
				if next == '\'' {
					i++
					builder.WriteByte(next)
				} else {
					inSingleQuote = false
				}
			}
		case inDoubleQuote:
			builder.WriteByte(ch)
			if ch == '"' {
				inDoubleQuote = false
			}
		case ch == '-' && next == '-':
			builder.WriteByte(ch)
			i++
			builder.WriteByte(next)
			inLineComment = true
		case ch == '/' && next == '*':
			builder.WriteByte(ch)
			i++
			builder.WriteByte(next)
			inBlockComment = true
		case ch == '\'':
			builder.WriteByte(ch)
			inSingleQuote = true
		case ch == '"':
			builder.WriteByte(ch)
			inDoubleQuote = true
		case ch == '?':
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(argIndex))
			argIndex++
		default:
			builder.WriteByte(ch)
		}
	}
	return builder.String()
}

func sqliteDSN(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_temp_store=MEMORY"
}
