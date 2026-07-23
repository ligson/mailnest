package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"mailnest-be/internal/storage"
)

var orderedTables = []string{
	"users",
	"mail_accounts",
	"mail_folders",
	"contacts",
	"mail_messages",
	"mail_message_states",
	"mail_rules",
	"mail_rule_conditions",
	"mail_attachments",
	"mail_sync_jobs",
	"mail_sync_job_events",
}

func main() {
	sqlitePath := flag.String("sqlite", "", "SQLite database path")
	mysqlDSN := flag.String("mysql", "", "MySQL DSN")
	flag.Parse()
	if strings.TrimSpace(*sqlitePath) == "" || strings.TrimSpace(*mysqlDSN) == "" {
		log.Fatal("必须同时提供 -sqlite 和 -mysql")
	}

	started := time.Now()
	log.Printf("迁移准备：初始化 MySQL schema")
	store, err := storage.OpenWithOptions(storage.DatabaseOptions{
		Driver:       "mysql",
		DSN:          *mysqlDSN,
		MaxOpenConns: 4,
		MaxIdleConns: 2,
	})
	if err != nil {
		log.Fatalf("初始化 MySQL schema 失败：%v", err)
	}
	_ = store.Close()

	sqliteDB, err := sql.Open("sqlite3", *sqlitePath+"?_busy_timeout=5000")
	if err != nil {
		log.Fatalf("打开 SQLite 失败：%v", err)
	}
	defer sqliteDB.Close()
	mysqlDB, err := sql.Open("mysql", *mysqlDSN)
	if err != nil {
		log.Fatalf("打开 MySQL 失败：%v", err)
	}
	defer mysqlDB.Close()

	ctx := context.Background()
	if err := sqliteDB.PingContext(ctx); err != nil {
		log.Fatalf("连接 SQLite 失败：%v", err)
	}
	if err := mysqlDB.PingContext(ctx); err != nil {
		log.Fatalf("连接 MySQL 失败：%v", err)
	}

	targetRows, err := totalRows(ctx, mysqlDB)
	if err != nil {
		log.Fatalf("检查 MySQL 是否为空失败：%v", err)
	}
	if targetRows != 0 {
		log.Fatalf("目标 MySQL 已有 %d 行数据，为避免覆盖，迁移中止", targetRows)
	}

	if _, err := mysqlDB.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		log.Fatalf("关闭 MySQL 外键检查失败：%v", err)
	}
	for _, table := range orderedTables {
		copied, err := copyTable(ctx, sqliteDB, mysqlDB, table)
		if err != nil {
			log.Fatalf("复制表 %s 失败：%v", table, err)
		}
		log.Printf("迁移表完成 table=%s rows=%d", table, copied)
	}
	if _, err := mysqlDB.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1"); err != nil {
		log.Fatalf("恢复 MySQL 外键检查失败：%v", err)
	}

	for _, table := range orderedTables {
		left, right, err := compareCount(ctx, sqliteDB, mysqlDB, table)
		if err != nil {
			log.Fatalf("校验表 %s 失败：%v", table, err)
		}
		if left != right {
			log.Fatalf("校验表 %s 不一致：sqlite=%d mysql=%d", table, left, right)
		}
		if err := resetAutoIncrement(ctx, mysqlDB, table); err != nil {
			log.Fatalf("重置表 %s 自增值失败：%v", table, err)
		}
		log.Printf("校验表通过 table=%s rows=%d", table, left)
	}
	log.Printf("迁移完成 tables=%d duration=%s", len(orderedTables), time.Since(started))
}

func copyTable(ctx context.Context, sqliteDB, mysqlDB *sql.DB, table string) (int64, error) {
	columns, err := sqliteColumns(ctx, sqliteDB, table)
	if err != nil {
		return 0, err
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("未读取到列")
	}
	query := fmt.Sprintf("SELECT %s FROM %s", joinQuoted(columns, "`"), table)
	rows, err := sqliteDB.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	placeholders := strings.TrimRight(strings.Repeat("?,", len(columns)), ",")
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, joinQuoted(columns, "`"), placeholders)
	tx, err := mysqlDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	values := make([]any, len(columns))
	scanDest := make([]any, len(columns))
	for i := range values {
		scanDest[i] = &values[i]
	}
	var copied int64
	for rows.Next() {
		for i := range values {
			values[i] = nil
		}
		if err := rows.Scan(scanDest...); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		for i, value := range values {
			values[i] = normalizeValue(value)
		}
		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		copied++
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return copied, nil
}

func sqliteColumns(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

func compareCount(ctx context.Context, sqliteDB, mysqlDB *sql.DB, table string) (int64, int64, error) {
	left, err := tableCount(ctx, sqliteDB, table)
	if err != nil {
		return 0, 0, err
	}
	right, err := tableCount(ctx, mysqlDB, table)
	if err != nil {
		return 0, 0, err
	}
	return left, right, nil
}

func tableCount(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func totalRows(ctx context.Context, db *sql.DB) (int64, error) {
	var total int64
	for _, table := range orderedTables {
		count, err := tableCount(ctx, db, table)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func resetAutoIncrement(ctx context.Context, db *sql.DB, table string) error {
	var maxID sql.NullInt64
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT MAX(id) FROM %s", table)).Scan(&maxID); err != nil {
		return err
	}
	next := int64(1)
	if maxID.Valid {
		next = maxID.Int64 + 1
	}
	_, err := db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s AUTO_INCREMENT = %d", table, next))
	return err
}

func normalizeValue(value any) any {
	switch v := value.(type) {
	case []byte:
		return string(v)
	case time.Time:
		if v.IsZero() {
			return nil
		}
		return v
	default:
		return v
	}
}

func joinQuoted(values []string, quote string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, quote+value+quote)
	}
	return strings.Join(out, ",")
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
}
