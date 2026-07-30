package mail

import "testing"

func TestParseTimeAcceptsDateWithTrailingTimezoneComment(t *testing.T) {
	parsed := parseTime("Mon, 8 Sep 2025 19:03:56 +0800 (GMT+08:00)")
	if !parsed.Valid {
		t.Fatal("expected mail Date with trailing timezone comment to parse")
	}
	if got := parsed.Time.Format("2006-01-02 15:04:05 -0700"); got != "2025-09-08 19:03:56 +0800" {
		t.Fatalf("unexpected parsed time: %s", got)
	}
}
