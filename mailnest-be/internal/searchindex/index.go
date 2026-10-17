// Package searchindex 维护可丢弃、可重建的邮件检索副本，数据库始终是正式数据源。
package searchindex

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"mailnest-be/internal/storage"
)

const formatVersion = 1

var contentFields = []string{"subject", "from", "to", "cc", "body"}

func indexMapping() *mapping.IndexMappingImpl {
	m := bleve.NewIndexMapping()
	d := bleve.NewDocumentStaticMapping()
	for _, name := range append(append([]string{}, contentFields...), "user", "uncertain") {
		f := bleve.NewKeywordFieldMapping()
		f.Store, f.IncludeTermVectors, f.IncludeInAll, f.DocValues = false, false, false, false
		f.SkipFreqNorm = true
		d.AddFieldMappingsAt(name, f)
	}
	m.DefaultMapping = d
	m.StoreDynamic, m.IndexDynamic, m.DocValuesDynamic = false, false, false
	return m
}

func documentKey(userID, messageID int64) string {
	return strconv.FormatInt(userID, 10) + "/" + strconv.FormatInt(messageID, 10)
}
func safeRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x4e00 && r <= 0x9fff {
		return r
	}
	return 0
}

// 去重的单字/双字集合保持中文片段、单字、数字和邮箱片段的子串候选语义。
// 不复制正文，不生成巨大的逐位置 token stream；最终仍由数据库 LIKE 精确复核。
func grams(value string) ([]string, bool) {
	seen := make(map[string]struct{})
	previous := rune(0)
	uncertain := false
	for _, original := range value {
		current := safeRune(original)
		if current == 0 {
			// 对非基本汉字/ASCII 的字母、数字、组合字符和不可见格式符保守放行，
			// 避免不同数据库 Unicode 排序规则的等价字符被候选层错误排除。
			if unicode.IsLetter(original) || unicode.IsNumber(original) || unicode.IsMark(original) || unicode.Is(unicode.Cf, original) {
				uncertain = true
			}
			previous = 0
			continue
		}
		seen[string(current)] = struct{}{}
		if previous != 0 {
			seen[string([]rune{previous, current})] = struct{}{}
		}
		previous = current
	}
	result := make([]string, 0, len(seen))
	for gram := range seen {
		result = append(result, gram)
	}
	return result, uncertain
}
func indexDocument(d storage.MailSearchDocument) map[string]any {
	result := map[string]any{"user": strconv.FormatInt(d.UserID, 10)}
	uncertain := []string{}
	for i, value := range []string{d.Subject, d.From, d.To, d.CC, d.Body} {
		tokens, unsafe := grams(value)
		result[contentFields[i]] = tokens
		if unsafe {
			uncertain = append(uncertain, contentFields[i])
		}
	}
	result["uncertain"] = uncertain
	return result
}

// 只选 LIKE 的必要条件，通配符/标点会断开片段，反斜线语义不确定时回退。
func queryGrams(value string) []string {
	if strings.Contains(value, `\`) {
		return nil
	}
	var singles, pairs []string
	seen := map[string]bool{}
	previous := rune(0)
	for _, r := range strings.TrimSpace(value) {
		current := safeRune(r)
		if current == 0 {
			previous = 0
			continue
		}
		single := string(current)
		if !seen[single] {
			seen[single] = true
			singles = append(singles, single)
		}
		if previous != 0 {
			pair := string([]rune{previous, current})
			if !seen[pair] {
				seen[pair] = true
				pairs = append(pairs, pair)
			}
		}
		previous = current
		if len(pairs) >= 8 {
			break
		}
	}
	if len(pairs) > 0 {
		return pairs
	}
	if len(singles) > 8 {
		singles = singles[:8]
	}
	return singles
}
