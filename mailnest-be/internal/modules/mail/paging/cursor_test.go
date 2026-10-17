package paging

import (
	"net/url"
	"strings"
	"testing"
)

func TestCursorBoundToUserAndFilters(t *testing.T) {
	q := url.Values{"keyword": {"测试601"}, "pageSize": {"20"}}
	scope := Scope(1, q)
	token := Encode(42, scope, "secret")
	q.Set("page", "2")
	q.Set("includeTotal", "false")
	if id, err := Decode(token, Scope(1, q), "secret"); err != nil || id != 42 {
		t.Fatalf("分页失败 %d %v", id, err)
	}
	for _, bad := range []struct{ token, scope, secret string }{
		{token, Scope(2, q), "secret"}, {token, scope, "other"}, {token + "x", scope, "secret"}, {strings.Repeat("x", 1025), scope, "secret"},
	} {
		if _, err := Decode(bad.token, bad.scope, bad.secret); err == nil {
			t.Fatal("接受了错误游标")
		}
	}
	q.Set("keyword", "其他")
	if _, err := Decode(token, Scope(1, q), "secret"); err == nil {
		t.Fatal("筛选未绑定")
	}
}
