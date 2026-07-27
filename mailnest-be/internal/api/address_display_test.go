package api

import (
	"database/sql"
	"testing"

	"mailnest-be/internal/storage"
)

func TestMailSendLogPayloadDecodesRecipientEncodedWords(t *testing.T) {
	item := storage.MailSendLog{
		ID:             1,
		AccountID:      2,
		RecipientsJSON: `{"to":["=?utf-8?q?=E6=B2=99=E8=88=9F=E7=8B=BC?= <ligson@aliyun.com>"],"cc":["=?utf-8?b?5rWL6K+V?= <copy@example.com>"],"bcc":[]}`,
		Subject:        "测试发信",
		Status:         "success",
		RetryStatus:    "none",
	}
	payload := mailSendLogPayload(item)
	recipients := payload["recipients"].(map[string][]string)

	if got := recipients["to"][0]; got != "沙舟狼 <ligson@aliyun.com>" {
		t.Fatalf("expected decoded to recipient, got %q", got)
	}
	if got := recipients["cc"][0]; got != "测试 <copy@example.com>" {
		t.Fatalf("expected decoded cc recipient, got %q", got)
	}
}

func TestMessageListPayloadDecodesAddressFields(t *testing.T) {
	item := storage.MailMessage{
		ID:        1,
		AccountID: 2,
		FromAddr:  sql.NullString{String: "=?utf-8?q?=E6=B2=99=E8=88=9F=E7=8B=BC?= <sender@example.com>", Valid: true},
		ToAddrs:   sql.NullString{String: "=?utf-8?b?5rWL6K+V?= <reader@example.com>", Valid: true},
	}
	payload := messageListPayload(item)
	to := payload["to"].([]string)

	if got := payload["from"]; got != "沙舟狼 <sender@example.com>" {
		t.Fatalf("expected decoded from address, got %#v", got)
	}
	if len(to) != 1 || to[0] != "测试 <reader@example.com>" {
		t.Fatalf("expected decoded to address, got %#v", to)
	}
}
