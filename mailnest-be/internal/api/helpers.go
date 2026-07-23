package api

import (
	"database/sql"
	"log"
	"net/http"
	netmail "net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"mailnest-be/internal/response"
)

func accountRouteIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return 0, 0, false
	}
	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮箱账号 ID 格式错误")
		return 0, 0, false
	}
	return userID, accountID, true
}

func parseOptionalID(w http.ResponseWriter, value string, label string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, label+"格式错误")
		return 0, false
	}
	return id, true
}

func parseOptionalIDs(w http.ResponseWriter, values []string, label string) ([]int64, bool) {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		id, ok := parseOptionalID(w, value, label)
		if !ok {
			return nil, false
		}
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, true
}

func normalizeOutgoingAddresses(w http.ResponseWriter, values []string, label string) ([]string, bool) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		addresses, err := netmail.ParseAddressList(value)
		if err != nil {
			response.Error(w, http.StatusBadRequest, label+"邮箱格式不正确")
			return nil, false
		}
		for _, address := range addresses {
			if strings.TrimSpace(address.Address) == "" {
				response.Error(w, http.StatusBadRequest, label+"邮箱格式不正确")
				return nil, false
			}
			out = append(out, address.String())
		}
	}
	if len(out) > 200 {
		response.Error(w, http.StatusBadRequest, label+"不能超过 200 个")
		return nil, false
	}
	return out, true
}

func normalizeSentFolder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Sent"
	}
	return value
}

func normalizeUITheme(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "sky", "grape", "ember", "graphite", "qinghua", "cinnabar", "ink", "daishan":
		return value
	default:
		return "forest"
	}
}

func nullableStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func readOptionalFile(value sql.NullString) string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return ""
	}
	content, err := os.ReadFile(value.String)
	if err != nil {
		return ""
	}
	return string(content)
}

func logSlowAPI(name string, started time.Time, details ...string) {
	duration := time.Since(started)
	if duration < 500*time.Millisecond {
		return
	}
	fields := []string{"慢接口", "name=" + name, "duration=" + duration.String()}
	fields = append(fields, details...)
	log.Print(strings.Join(fields, " "))
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseDateQuery(value string) sql.NullTime {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullTime{}
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: parsed, Valid: true}
}

func parseBoolQuery(value string) sql.NullBool {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "true", "1", "yes":
		return sql.NullBool{Bool: true, Valid: true}
	case "false", "0", "no":
		return sql.NullBool{Bool: false, Valid: true}
	default:
		return sql.NullBool{}
	}
}

func parseIDStrings(values []string) ([]int64, bool) {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func sqlNullInt64(value int64) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
}

func valueOrFallback(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
