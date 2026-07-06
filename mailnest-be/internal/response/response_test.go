package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONWritesUnifiedEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	JSON(rec, http.StatusCreated, true, "创建成功", map[string]string{"id": "user-id"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var body Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !body.Success {
		t.Fatal("expected success true")
	}
	if body.Message != "创建成功" {
		t.Fatalf("expected message 创建成功, got %q", body.Message)
	}
	if body.HTTPCode != http.StatusCreated {
		t.Fatalf("expected httpCode 201, got %d", body.HTTPCode)
	}
	if body.Data == nil {
		t.Fatal("expected data to be present")
	}
}

func TestJSONUsesEmptyObjectWhenDataIsNil(t *testing.T) {
	rec := httptest.NewRecorder()

	JSON(rec, http.StatusOK, true, "操作成功", nil)

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data, ok := raw["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be an object, got %#v", raw["data"])
	}
	if len(data) != 0 {
		t.Fatalf("expected empty data object, got %#v", data)
	}
}
