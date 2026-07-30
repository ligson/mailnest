package mail

import (
	"database/sql"
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"time"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

var htmlScriptPattern = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)

var htmlEventAttrPattern = regexp.MustCompile(`(?is)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)

var htmlJavascriptURLPattern = regexp.MustCompile(`(?is)(href|src)\s*=\s*("|')\s*javascript:[^"']*("|')`)

var htmlImageTagPattern = regexp.MustCompile(`(?is)<img\b[^>]*>`)

var mailDateTrailingCommentPattern = regexp.MustCompile(`\s*\([^)]*\)\s*$`)

func buildSearchText(fetched FetchedMessage, toAddrs, ccAddrs string) string {
	parts := []string{
		fetched.TextBody,
		stripHTMLTags(fetched.HTMLBody),
	}
	return strings.Join(parts, "\n")
}

func stripHTMLTags(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	withoutTags := htmlTagPattern.ReplaceAllString(value, " ")
	return html.UnescapeString(withoutTags)
}

func stripUnsafeQuoteHTML(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	value = htmlScriptPattern.ReplaceAllString(value, "")
	value = htmlEventAttrPattern.ReplaceAllString(value, "")
	value = htmlJavascriptURLPattern.ReplaceAllString(value, `$1="#"`)
	value = htmlImageTagPattern.ReplaceAllString(value, `<span style="color:#8c8c8c;">[内嵌图片已省略]</span>`)
	return value
}

func valueOrExisting(value, existing string) string {
	if strings.TrimSpace(value) == "" {
		return existing
	}
	return value
}

func parseTime(value string) sql.NullTime {
	value = normalizeMailDateValue(value)
	if value == "" {
		return sql.NullTime{}
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 MST",
	} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return sql.NullTime{Time: parsed, Valid: true}
		}
	}
	return sql.NullTime{}
}

func normalizeMailDateValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "(") && strings.Contains(value, ")") {
		if cleaned := strings.TrimSpace(mailDateTrailingCommentPattern.ReplaceAllString(value, "")); cleaned != "" {
			return cleaned
		}
	}
	return value
}

func safePath(value string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_")
	return replacer.Replace(value)
}

func mustJSON(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}
