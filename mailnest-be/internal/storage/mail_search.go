package storage

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const mailSearchIndexName = "ft_mail_messages_ngram_v1"
const mailSearchIndexVersion = "mailnest-ngram-2-default-stopwords-v1"
const mailSearchColumns = "m.subject, m.from_addr, m.to_addrs, m.cc_addrs, m.search_text"

type mailSearchCapabilities struct {
	ngram     bool
	stopwords []string
}

// mysqlSearchEnvironment 拒绝自定义停用词配置，避免索引与查询分词不一致而漏信。
func (s *Store) mysqlSearchEnvironment(ctx context.Context) ([]string, error) {
	var size int
	var custom string
	if err := s.db.QueryRowContext(ctx, `SELECT @@ngram_token_size, CONCAT(COALESCE(@@global.innodb_ft_server_stopword_table, ''), COALESCE(@@global.innodb_ft_user_stopword_table, ''))`).Scan(&size, &custom); err != nil {
		return nil, err
	}
	if size != 2 || custom != "" {
		return nil, fmt.Errorf("全文检索要求 ngram_token_size=2 且未配置全局自定义停用词表")
	}
	var caseInsensitiveColumns int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='mail_messages' AND COLUMN_NAME IN ('subject','from_addr','to_addrs','cc_addrs','search_text') AND RIGHT(COLLATION_NAME,3)='_ci'`).Scan(&caseInsensitiveColumns); err != nil {
		return nil, err
	}
	if caseInsensitiveColumns != 5 {
		return nil, fmt.Errorf("全文候选检索要求邮件搜索字段使用不区分大小写的排序规则")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT value FROM information_schema.INNODB_FT_DEFAULT_STOPWORD`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var words []string
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return nil, err
		}
		words = append(words, strings.ToLower(word))
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("无法读取默认停用词，保留兼容搜索以防遗漏邮件")
	}
	return words, rows.Err()
}

func (s *Store) detectMailSearchIndex() {
	if s.db.dialect != dialectMySQL {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	words, err := s.mysqlSearchEnvironment(ctx)
	if err != nil {
		log.Printf("邮件搜索使用兼容模式，无法启用 ngram：%v", err)
		return
	}
	var columns string
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX), '')
		FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='mail_messages'
		AND INDEX_NAME=? AND INDEX_TYPE='FULLTEXT' AND INDEX_COMMENT=?`, mailSearchIndexName, mailSearchIndexVersion).Scan(&columns)
	if err != nil || columns != "subject,from_addr,to_addrs,cc_addrs,search_text" {
		log.Print("邮件搜索使用兼容模式；执行 -prepare-search-index 可准备 MySQL 全文索引")
		return
	}
	var table, definition string
	if err := s.db.QueryRowContext(ctx, `SHOW CREATE TABLE mail_messages`).Scan(&table, &definition); err != nil {
		log.Print("无法确认邮件全文索引解析器，保留兼容搜索")
		return
	}
	parserVerified := false
	for _, line := range strings.Split(definition, "\n") {
		if strings.Contains(line, "`"+mailSearchIndexName+"`") && strings.Contains(line, "WITH PARSER `ngram`") {
			parserVerified = true
		}
	}
	if !parserVerified {
		log.Print("邮件全文索引未使用 ngram 解析器，保留兼容搜索")
		return
	}
	s.mailSearch = mailSearchCapabilities{ngram: true, stopwords: words}
	log.Print("邮件搜索已启用 MySQL ngram 全文候选检索及原始字段复核")
}

// MailSearchMode 只暴露引擎类型，用于耗时日志，不记录用户搜索内容。
func (s *Store) MailSearchMode() string {
	if s.searchProvider != nil {
		return s.searchProvider.Mode()
	}
	if s.mailSearch.ngram {
		return "mysql-ngram"
	}
	return "like"
}

// PrepareMailSearchIndex 仅供显式运维命令调用；普通启动不执行大表建索引。
// 只新增可删除的派生索引，不改写邮件、凭据或正文文件。
func (s *Store) PrepareMailSearchIndex(ctx context.Context) error {
	if s.db.dialect != dialectMySQL {
		return fmt.Errorf("此命令仅支持 MySQL ngram 索引，其他数据库保持兼容搜索")
	}
	if _, err := s.mysqlSearchEnvironment(ctx); err != nil {
		return err
	}
	if s.mailSearch.ngram {
		return nil
	}
	var existing int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='mail_messages' AND INDEX_NAME=?`, mailSearchIndexName).Scan(&existing); err != nil {
		return err
	}
	if existing != 0 {
		return fmt.Errorf("同名索引不符合预期，请人工核查，禁止自动覆盖")
	}
	conn, err := s.db.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	// 使用默认停用词，并在专用连接限制元数据锁等待，不改变共享 MySQL 全局设置。
	if _, err := conn.ExecContext(ctx, `SET SESSION lock_wait_timeout=15, innodb_ft_enable_stopword=ON, innodb_ft_user_stopword_table=DEFAULT`); err != nil {
		return err
	}
	log.Print("开始构建邮件 ngram 全文索引，请确保已完成数据库备份")
	_, err = conn.ExecContext(ctx, `ALTER TABLE mail_messages ADD FULLTEXT INDEX ft_mail_messages_ngram_v1 (subject, from_addr, to_addrs, cc_addrs, search_text) WITH PARSER ngram COMMENT 'mailnest-ngram-2-default-stopwords-v1'`)
	// 归还连接前恢复会话设置，避免影响同一连接池的其他运维操作。
	resetCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, resetErr := conn.ExecContext(resetCtx, `SET SESSION lock_wait_timeout=DEFAULT, innodb_ft_enable_stopword=DEFAULT, innodb_ft_user_stopword_table=DEFAULT`)
	if err != nil {
		return err
	}
	if resetErr != nil {
		return resetErr
	}
	if err := conn.Close(); err != nil {
		return err
	}
	s.detectMailSearchIndex()
	if !s.mailSearch.ngram {
		return fmt.Errorf("索引已创建但能力检查未通过，请检查数据库配置")
	}
	return nil
}

// 必须是 LIKE 匹配的必要条件：不直接把用户文本交给布尔检索解析器。
// 跳过停用词、标点、通配符和无法确定分词语义的字符；无法提取时回退 LIKE。
func mailSearchTerms(value string, stopwords []string) string {
	if strings.Contains(value, `\`) {
		return ""
	}
	var terms []string
	seen := map[string]bool{}
	var previous rune
	for _, current := range value {
		if current >= 'A' && current <= 'Z' {
			current += 'a' - 'A'
		}
		safe := current >= 'a' && current <= 'z' || current >= '0' && current <= '9' || current >= '\u4e00' && current <= '\u9fff'
		if !safe {
			previous = 0
			continue
		}
		if previous != 0 {
			gram := string([]rune{previous, current})
			blocked := false
			for _, word := range stopwords {
				if word != "" && strings.Contains(gram, word) {
					blocked = true
					break
				}
			}
			if !blocked && !seen[gram] {
				terms = append(terms, `+"`+gram+`"`)
				seen[gram] = true
				if len(terms) == 8 {
					break
				}
			}
		}
		previous = current
	}
	return strings.Join(terms, " ")
}

// 邮件列表和会话列表共用条件；外层查询始终保留 user_id 和状态隔离。
func (s *Store) mailSearchWhere(keyword, from, subject, body string) (string, []any) {
	var where strings.Builder
	var args []any
	for index, value := range []string{keyword, from, subject, body} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if s.mailSearch.ngram && s.searchProvider == nil {
			if terms := mailSearchTerms(value, s.mailSearch.stopwords); terms != "" {
				where.WriteString(" AND MATCH(" + mailSearchColumns + ") AGAINST (? IN BOOLEAN MODE)")
				args = append(args, terms)
			}
		}
		like := "%" + value + "%"
		if index == 0 {
			where.WriteString(` AND (COALESCE(m.subject, '') LIKE ? OR COALESCE(m.from_addr, '') LIKE ? OR COALESCE(m.to_addrs, '') LIKE ? OR COALESCE(m.cc_addrs, '') LIKE ? OR COALESCE(m.search_text, '') LIKE ?)`)
			args = append(args, like, like, like, like, like)
		} else {
			column := []string{"", "from_addr", "subject", "search_text"}[index]
			where.WriteString(" AND COALESCE(m." + column + ", '') LIKE ?")
			args = append(args, like)
		}
	}
	return where.String(), args
}
