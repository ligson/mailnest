package api

import (
	"encoding/json"
	"testing"
)

func TestContactsBatchAPI(t *testing.T) {
	router := newTestRouter(t, true)
	owner := registerTestUser(t, router, "batch-api", "batch-api@example.com")
	other := registerTestUser(t, router, "batch-other", "batch-other@example.com")
	request := func(method, path string, payload any, token string, status int) map[string]any {
		t.Helper()
		body := ""
		if raw, ok := payload.(string); ok {
			body = raw
		} else if payload != nil {
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			body = string(raw)
		}
		response := performRequest(router, method, path, body, token)
		if response.Code != status {
			t.Fatalf("%s %s: %d: %s", method, path, response.Code, response.Body.String())
		}
		envelope := decodeEnvelope(t, response.Body.Bytes())
		if envelope["httpCode"] != float64(status) || envelope["success"] != (status < 400) {
			t.Fatalf("响应封装错误: %+v", envelope)
		}
		data, ok := envelope["data"].(map[string]any)
		if !ok {
			t.Fatal("data 必须为对象")
		}
		return data
	}
	path := "/api/v1/contacts/batch"
	request("PATCH", path, nil, "", 401)
	contact := request("POST", "/api/v1/contacts", map[string]any{"email": "a@example.com", "nickname": "昵称", "company": "旧公司"}, owner, 201)
	foreign := request("POST", "/api/v1/contacts", map[string]any{"email": "b@example.com"}, other, 201)
	group := request("POST", "/api/v1/contact-groups", map[string]any{"name": "项目组"}, owner, 201)
	foreignGroup := request("POST", "/api/v1/contact-groups", map[string]any{"name": "他人分组"}, other, 201)
	payload := func(ids []any) map[string]any {
		return map[string]any{"contactIds": ids, "company": map[string]any{"mode": "set", "value": "新公司"}}
	}
	ids := []any{contact["id"]}
	for _, body := range []string{
		`{"contactIds":["wrong"],"company":{"mode":"set","value":"公司"}}`,
		`{"contactIds":["1"],"unknown":true}`,
		`{"contactIds":["1"]} {}`,
		`{"contactIds":[]}`,
	} {
		request("PATCH", path, body, owner, 400)
	}
	request("PATCH", path, map[string]any{"contactIds": ids}, owner, 400)
	request("PATCH", path, payload([]any{contact["id"], foreign["id"]}), owner, 400)
	combined := payload(ids)
	combined["groups"] = map[string]any{"mode": "add", "groupIds": []any{group["id"], foreignGroup["id"]}}
	request("PATCH", path, combined, owner, 404)
	before := request("GET", "/api/v1/contacts", nil, owner, 200)["items"].([]any)[0].(map[string]any)
	if before["company"] != "旧公司" {
		t.Fatal("失败操作未完整回滚", before)
	}
	combined["groups"] = map[string]any{"mode": "add", "groupIds": []any{group["id"]}}
	result := request("PATCH", path, combined, owner, 200)
	if result["selectedCount"] != float64(1) || result["companyUpdatedCount"] != float64(1) || result["groupMembershipChangedCount"] != float64(1) {
		t.Fatal("结果计数错误", result)
	}
	result = request("PATCH", path, combined, owner, 200)
	if result["companyUpdatedCount"] != float64(0) || result["groupMembershipChangedCount"] != float64(0) {
		t.Fatal("重试不应重复写入", result)
	}
	after := request("GET", "/api/v1/contacts", nil, owner, 200)["items"].([]any)[0].(map[string]any)
	if after["company"] != "新公司" || after["nickname"] != "昵称" {
		t.Fatal("修改公司影响其他资料", after)
	}
	request("PATCH", path, map[string]any{"contactIds": ids, "groups": map[string]any{"mode": "remove", "groupIds": []any{group["id"]}}}, owner, 200)
	detail := request("GET", "/api/v1/contact-groups/"+group["id"].(string), nil, owner, 200)
	if len(detail["members"].([]any)) != 0 {
		t.Fatal("移出分组失败")
	}
}
