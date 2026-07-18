package mail

import (
	"strings"
	"testing"
)

func TestParseBodiesExtractsAttachmentsAndInlineImages(t *testing.T) {
	raw := strings.Join([]string{
		"Subject: MIME test",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="mixed"`,
		"",
		"--mixed",
		`Content-Type: multipart/related; boundary="related"`,
		"",
		"--related",
		`Content-Type: text/html; charset="utf-8"`,
		"",
		`<p>Hello</p><img src="cid:inline-image-1">`,
		"--related",
		"Content-Type: image/png",
		"Content-Transfer-Encoding: base64",
		"Content-Disposition: inline",
		"Content-ID: <inline-image-1>",
		"",
		"aW5saW5lLWltYWdlLWJ5dGVz",
		"--related--",
		"--mixed",
		"Content-Type: application/pdf",
		"Content-Transfer-Encoding: base64",
		`Content-Disposition: attachment; filename="report.pdf"`,
		"",
		"JVBERi0xLjQ=",
		"--mixed--",
		"",
	}, "\r\n")

	_, htmlBody, attachments := parseBodies([]byte(raw))

	if !strings.Contains(htmlBody, `cid:inline-image-1`) {
		t.Fatalf("expected html body with cid image, got %q", htmlBody)
	}
	if len(attachments) != 2 {
		t.Fatalf("expected two attachments, got %#v", attachments)
	}
	if attachments[0].ContentID != "inline-image-1" || !attachments[0].Inline || string(attachments[0].Data) != "inline-image-bytes" {
		t.Fatalf("expected inline image attachment, got %#v", attachments[0])
	}
	if attachments[1].Filename != "report.pdf" || attachments[1].Inline || string(attachments[1].Data) != "%PDF-1.4" {
		t.Fatalf("expected normal pdf attachment, got %#v", attachments[1])
	}
}

func TestFetchedMessageFromRawDecodesGBKHeadersAndBody(t *testing.T) {
	raw := strings.Join([]string{
		"Message-ID: <gbk-message@example.com>",
		"Subject: =?gbk?b?vK/Nxc34wuewssirsr/P3sba1fu4xM2o1qq6rw==?=",
		"From: =?gbk?b?1cXI/Q==?= <sender@example.com>",
		"To: =?gbk?b?ytW8/sjL?= <to@example.com>",
		"Date: Wed, 08 Jul 2026 08:42:17 +0800",
		"MIME-Version: 1.0",
		`Content-Type: text/plain; charset="gbk"`,
		"Content-Transfer-Encoding: base64",
		"",
		"w7vT0NX9zsTE2sjd",
		"",
	}, "\r\n")

	message := fetchedMessageFromRaw([]byte(raw))

	if message.Subject != "集团网络安全部限期整改通知函" {
		t.Fatalf("expected decoded subject, got %q", message.Subject)
	}
	if message.From != "张三 <sender@example.com>" {
		t.Fatalf("expected decoded from, got %q", message.From)
	}
	if len(message.To) != 1 || message.To[0] != "收件人 <to@example.com>" {
		t.Fatalf("expected decoded to, got %#v", message.To)
	}
	if strings.TrimSpace(message.TextBody) != "没有正文内容" {
		t.Fatalf("expected decoded text body, got %q", message.TextBody)
	}
}

func TestParseMessageBodiesRepairsMissingHeaderBodySeparator(t *testing.T) {
	raw := strings.Join([]string{
		"From: =?gbk?b?1tC5+rGxvqnSxravuavLvg==?= <10086@139.com>",
		"To: ligson@aliyun.com <ligson@aliyun.com>",
		"Subject: =?gbk?b?ob7Sxravt6LGsaG/xPq1xLXn19O3osaxob42MjQxMjQwMjkwob/S0cvNtO+jrL60x+uy6dTEo6E=?=",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed;`,
		`  boundary="=_outer"`,
		"--=_outer",
		"Content-Type: multipart/alternative;",
		`  boundary="=_inner"`,
		"Content-Transfer-Encoding: 7bit",
		"",
		"--=_inner",
		`Content-Type: text/plain; charset="gbk"`,
		"Content-Transfer-Encoding: base64",
		"",
		"w7vT0NX9zsTE2sjd",
		"--=_inner--",
		"--=_outer--",
		"",
	}, "\r\n")

	textBody, htmlBody, attachments := parseMessageBodies([]byte(raw))

	if strings.TrimSpace(textBody) != "没有正文内容" {
		t.Fatalf("expected decoded text body, got %q", textBody)
	}
	if strings.Contains(strings.ToLower(textBody), "content-type:") || strings.Contains(textBody, "--=_") {
		t.Fatalf("expected clean body without MIME boundaries, got %q", textBody)
	}
	if htmlBody != "" {
		t.Fatalf("expected empty html body, got %q", htmlBody)
	}
	if len(attachments) != 0 {
		t.Fatalf("expected no attachments, got %#v", attachments)
	}
}

func TestParseBodiesDecodesGBKAttachmentFilename(t *testing.T) {
	raw := strings.Join([]string{
		"Subject: Attachment test",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="mixed"`,
		"",
		"--mixed",
		`Content-Type: text/plain; charset="utf-8"`,
		"",
		"hello",
		"--mixed",
		"Content-Type: application/octet-stream",
		"Content-Transfer-Encoding: base64",
		`Content-Disposition: attachment; filename="=?gbk?b?suLK1Li9vP4udHh0?="`,
		"",
		"ZmlsZQ==",
		"--mixed--",
		"",
	}, "\r\n")

	_, _, attachments := parseBodies([]byte(raw))

	if len(attachments) != 1 {
		t.Fatalf("expected one attachment, got %#v", attachments)
	}
	if attachments[0].Filename != "测试附件.txt" {
		t.Fatalf("expected decoded attachment filename, got %q", attachments[0].Filename)
	}
}
