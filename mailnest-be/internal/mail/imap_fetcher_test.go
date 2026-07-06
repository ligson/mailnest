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
