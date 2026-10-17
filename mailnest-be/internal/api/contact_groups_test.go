package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestContactGroupAPIEnvelopeAndIsolation(t *testing.T) {
	router := newTestRouter(t, true)
	owner := registerTestUser(t, router, "group-owner", "group-owner@example.com")
	other := registerTestUser(t, router, "group-other", "group-other@example.com")
	request := func(method, path string, payload any, token string, status int) map[string]any {
		t.Helper()
		body := ""
		if payload != nil {
			raw, e := json.Marshal(payload)
			if e != nil {
				t.Fatal(e)
			}
			body = string(raw)
		}
		r := performRequest(router, method, path, body, token)
		if r.Code != status {
			t.Fatalf("%s %s: expected %d got %d: %s", method, path, status, r.Code, r.Body.String())
		}
		envelope := decodeEnvelope(t, r.Body.Bytes())
		if envelope["httpCode"] != float64(status) || envelope["success"] != (status < 400) {
			t.Fatalf("envelope 错误: %+v", envelope)
		}
		data, ok := envelope["data"].(map[string]any)
		if !ok {
			t.Fatal("data 必须为对象")
		}
		return data
	}
	request("GET", "/api/v1/contact-groups", nil, "", 401)
	c := request("POST", "/api/v1/contacts", map[string]any{"email": "a@example.com", "nickname": "张海涛"}, owner, 201)
	foreign := request("POST", "/api/v1/contacts", map[string]any{"email": "foreign@example.com"}, other, 201)
	group := request("POST", "/api/v1/contact-groups", map[string]any{"name": "同事", "contactIds": []any{c["id"], c["id"]}}, owner, 201)
	id := group["id"].(string)
	detail := request("GET", "/api/v1/contact-groups/"+id, nil, owner, 200)
	if len(detail["members"].([]any)) != 1 {
		t.Fatal("成员未去重")
	}
	request("GET", "/api/v1/contact-groups/"+id, nil, other, 404)
	request("DELETE", "/api/v1/contact-groups/"+id, nil, other, 404)
	request("GET", "/api/v1/contacts?groupId="+id, nil, other, 404)
	request("GET", "/api/v1/contacts?groupId=wrong", nil, owner, 400)
	request("POST", "/api/v1/contact-groups", map[string]any{"name": "同事"}, owner, 409)
	request("POST", "/api/v1/contact-groups", map[string]any{"name": "跨用户", "contactIds": []any{foreign["id"]}}, owner, 400)
	request("POST", "/api/v1/contact-groups/"+id+"/members", map[string]any{"action": "add", "contactIds": []any{foreign["id"]}}, owner, 400)
	request("POST", "/api/v1/contact-groups/"+id+"/members", map[string]any{"action": "unknown", "contactIds": []any{c["id"]}}, owner, 400)
	request("PUT", "/api/v1/contact-groups/"+id, map[string]any{"name": "修改", "revision": 0}, owner, 409)
	request("PUT", "/api/v1/contact-groups/"+id, map[string]any{"name": "修改", "revision": group["revision"], "contactIds": []any{c["id"]}}, owner, 200)
	resolved := request("POST", "/api/v1/contact-groups/resolve", map[string]any{"groupIds": []string{id, id}}, owner, 200)
	items := resolved["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "张海涛" {
		t.Fatal("展开地址或昵称错误", items)
	}
	request("POST", "/api/v1/contact-groups/resolve", map[string]any{"groupIds": []string{id}}, other, 404)
	request("POST", "/api/v1/contact-groups/resolve", map[string]any{"groupIds": []string{"bad"}}, owner, 400)
	request("POST", "/api/v1/contact-groups", map[string]any{"name": "   "}, owner, 400)
	request("POST", "/api/v1/contact-groups/"+id+"/members", map[string]any{"action": "remove", "contactIds": []any{c["id"]}}, owner, 200)
	empty := request("POST", "/api/v1/contact-groups/resolve", map[string]any{"groupIds": []string{id}}, owner, 200)
	if len(empty["items"].([]any)) != 0 {
		t.Fatal("空组展开错误")
	}
	request("DELETE", "/api/v1/contact-groups/"+id, nil, owner, 200)
	request("GET", "/api/v1/contacts", nil, owner, http.StatusOK)
}
