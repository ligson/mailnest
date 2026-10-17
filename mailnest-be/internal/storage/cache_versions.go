package storage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type cacheMutation struct {
	userID int64
	domain string
}
type cacheVersions struct {
	mu     sync.Mutex
	global map[string]uint64
	users  map[cacheMutation]uint64
}

func (v *cacheVersions) key(userID int64, domain string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return fmt.Sprintf("%d-%d", v.global[domain], v.users[cacheMutation{userID, domain}])
}

func (v *cacheVersions) bump(change cacheMutation) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.global == nil {
		v.global = map[string]uint64{}
		v.users = map[cacheMutation]uint64{}
	}
	if change.userID <= 0 {
		v.global[change.domain]++
		return
	}
	if len(v.users) >= 4096 {
		// 有界用户版本表重置时提升全局代次，不能使旧缓存键重新生效。
		v.global["mail"]++
		v.global["contacts"]++
		v.users = map[cacheMutation]uint64{}
	}
	v.users[change]++
}

var userPlaceholder = regexp.MustCompile(`(?i)\b(?:[a-z_]+\.)?user_id\s*=\s*\?`)
var insertColumns = regexp.MustCompile(`(?is)^\s*insert(?:\s+or\s+ignore|\s+ignore)?\s+into\s+\w+\s*\(([^)]+)\)\s*values\s*\(([^)]+)\)`)
var uncertainMutation = regexp.MustCompile(`(?i)\b(or|select|union|join|returning)\b|[';]|--|/\*`)

// 只识别本项目参数化 SQL 中可证明的用户归属；无法确定时保守失效整个业务域。
// 不从日志/任务写入失效邮件，不猜测 ID 所属用户，不解析用户输入字符串。
func queryCacheMutation(query string, args []any) cacheMutation {
	query = strings.Join(strings.Fields(query), " ")
	query = strings.ReplaceAll(strings.ToLower(query), "insert or ignore", "insert ignore")
	domain := "mail"
	fields := strings.Fields(strings.ToLower(query))
	for _, field := range fields[:min(len(fields), 6)] {
		if strings.Trim(field, "\u0060\"()") == "contacts" {
			domain = "contacts"
			break
		}
	}
	change := cacheMutation{domain: domain}
	// 不尝试推断子查询、多用户 OR、多语句或字符串常量内的占位符。
	// 边界不明确时多失效缓存，比漏失效返回旧数据更安全。
	if uncertainMutation.MatchString(query) {
		return change
	}
	if match := insertColumns.FindStringSubmatch(query); len(match) > 0 {
		location := insertColumns.FindStringIndex(query)
		if strings.TrimSpace(query[location[1]:]) != "" {
			return change
		}
		columns := strings.Split(match[1], ",")
		values := strings.Split(match[2], ",")
		index := 0
		for i, column := range columns {
			if i >= len(values) {
				return change
			}
			if strings.TrimSpace(strings.Trim(column, "\u0060\" ")) == "user_id" && strings.TrimSpace(values[i]) == "?" {
				if index < len(args) {
					change.userID = positiveUserID(args[index])
				}
				return change
			}
			index += strings.Count(values[i], "?")
		}
		return change
	}
	where := strings.Index(strings.ToLower(query), " where ")
	matches := userPlaceholder.FindAllStringIndex(query, -1)
	if len(matches) == 1 && where >= 0 && matches[0][0] > where && !strings.ContainsAny(query[where:], "()") {
		match := matches[0]
		index := strings.Count(query[:match[1]], "?") - 1
		if index >= 0 && index < len(args) {
			change.userID = positiveUserID(args[index])
		}
	}
	return change
}

func positiveUserID(value any) int64 {
	var id int64
	switch v := value.(type) {
	case int64:
		id = v
	case int:
		id = int64(v)
	case string:
		id, _ = strconv.ParseInt(v, 10, 64)
	}
	if id < 0 {
		return 0
	}
	return id
}

func (db *database) invalidateQuery(query string, args []any) {
	db.mailRevision.Add(1)
	db.cacheVersions.bump(queryCacheMutation(query, args))
}

func (tx *transaction) recordCacheChange(query string, args []any) {
	if tx.cacheChanges == nil {
		tx.cacheChanges = map[cacheMutation]struct{}{}
	}
	tx.cacheChanges[queryCacheMutation(query, args)] = struct{}{}
}

type contactResolveQuery struct {
	UserID int64
	Keys   []string
}

func cacheQueryUser(query any) int64 {
	switch q := query.(type) {
	case ListMailMessagesQuery:
		return q.UserID
	case ListMailThreadsQuery:
		return q.UserID
	case MailSearchTextQuery:
		return q.UserID
	case contactResolveQuery:
		return q.UserID
	}
	return 0
}
