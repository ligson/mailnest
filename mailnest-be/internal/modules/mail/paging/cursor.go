// Package paging 保存邮件菜单的分页协议，不依赖 HTTP 装配层或数据库。
package paging

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

type boundary struct {
	ID    int64  `json:"id"`
	Scope string `json:"scope"`
}

// Scope 绑定用户与筛选条件，不能把其他用户或其他搜索的游标套用到当前列表。
func Scope(userID int64, values url.Values) string {
	copy := url.Values{}
	for key, values := range values {
		switch key {
		case "page", "cursor", "includeTotal", "deferred":
			continue
		}
		copy[key] = append([]string{}, values...)
	}
	hash := sha256.Sum256([]byte(strconv.FormatInt(userID, 10) + "/" + copy.Encode()))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func Encode(id int64, scope, secret string) string {
	data, _ := json.Marshal(boundary{ID: id, Scope: scope})
	body := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func Decode(token, scope, secret string) (int64, error) {
	invalid := errors.New("分页位置无效，请返回第一页后重试")
	if len(token) > 1024 {
		return 0, invalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, invalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, invalid
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return 0, invalid
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	var b boundary
	if err != nil || json.Unmarshal(data, &b) != nil || b.ID <= 0 || b.Scope != scope {
		return 0, invalid
	}
	return b.ID, nil
}
