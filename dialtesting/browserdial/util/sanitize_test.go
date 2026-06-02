package util

import (
	"strings"
	"testing"
	"time"
)

func TestSanitizeKey(t *testing.T) {
	tests := map[string]string{
		"flow.name": "flow_name",
		"bad key":   "bad_key",
		"9bad":      "k_9bad",
		"":          "key",
	}
	for input, want := range tests {
		if got := SanitizeKey(input, "key"); got != want {
			t.Fatalf("SanitizeKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSanitizeFields(t *testing.T) {
	got := SanitizeFields(map[string]any{
		"response.time": 12,
		"skip":          nil,
	})
	if got["response_time"] != 12 {
		t.Fatalf("expected response_time field, got %#v", got)
	}
	if _, ok := got["skip"]; ok {
		t.Fatalf("nil field should be skipped: %#v", got)
	}
}

func TestJSONString(t *testing.T) {
	got := JSONString(map[string]string{"hello": "world"}, 100)
	if got != `{"hello":"world"}` {
		t.Fatalf("JSONString = %s", got)
	}
}

func TestSanitizeTagsTruncateJSONStringAndTiming(t *testing.T) {
	tags := SanitizeTags(map[string]string{"Node Name": "shanghai", "1bad": "value"})
	if tags["node_name"] != "shanghai" || tags["k_1bad"] != "value" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	if got := Truncate("abcdef", 3); got != "abc" {
		t.Fatalf("unexpected short truncate: %q", got)
	}
	if got := Truncate("abcdefghijklmnopqrstuvwxyz", 20); got != "abcdef...[truncated]" {
		t.Fatalf("unexpected truncate suffix: %q", got)
	}
	if got := JSONString(func() {}, 100); !strings.Contains(got, "unsupported type") {
		t.Fatalf("unexpected JSON error string: %q", got)
	}
	if NowISO() == "" {
		t.Fatal("NowISO should not be empty")
	}
	if DurationUS(time.Now().Add(-time.Millisecond)) <= 0 {
		t.Fatal("DurationUS should be positive")
	}
	if id := NewRunID(); len(id) < 30 {
		t.Fatalf("unexpected run id %q", id)
	}
}
