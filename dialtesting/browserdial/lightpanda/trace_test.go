package lightpanda

import (
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestExtractTraceIDFromTraceparentHeader(t *testing.T) {
	got := extractTraceID("https://example.com", network.Headers{
		"Traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	})
	if got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace id = %q", got)
	}
}

func TestExtractTraceIDFromQuery(t *testing.T) {
	got := extractTraceID("https://example.com/api?foo=bar&trace_id=query-trace-1")
	if got != "query-trace-1" {
		t.Fatalf("trace id = %q", got)
	}
}

func TestExtractTraceIDPrefersHeaders(t *testing.T) {
	got := extractTraceID("https://example.com/api?traceid=query-trace", network.Headers{
		"x-b3-traceid": "header-trace",
	})
	if got != "header-trace" {
		t.Fatalf("trace id = %q", got)
	}
}

func TestExtractTraceIDFromUberTraceID(t *testing.T) {
	got := extractTraceID("https://example.com", network.Headers{
		"uber-trace-id": "abc123:def456:0:1",
	})
	if got != "abc123" {
		t.Fatalf("trace id = %q", got)
	}
}

func TestTraceHelpersCoverValueShapes(t *testing.T) {
	if got := extractTraceID("://bad-url", network.Headers{}); got != "" {
		t.Fatalf("bad URL trace id = %q", got)
	}
	if got := extractTraceID("https://example.com?x_trace_id=%27quoted%27"); got != "quoted" {
		t.Fatalf("quoted query trace id = %q", got)
	}
	if got := extractTraceIDFromHeaders(network.Headers{"x-trace-id": []string{"a", "b"}}); got != "a" {
		t.Fatalf("slice header trace id = %q", got)
	}
	if got := extractTraceIDFromHeaders(network.Headers{"dd-trace-id": []any{"c", "d"}}); got != "c" {
		t.Fatalf("any slice header trace id = %q", got)
	}
	if got := extractTraceIDFromHeaders(network.Headers{"x-traceid": nil}); got != "" {
		t.Fatalf("nil header trace id = %q", got)
	}
	if got := normalizeTraceValue("traceparent", "4bf92f3577b34da6a3ce929d0e0e4736"); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("fallback traceparent trace id = %q", got)
	}
	if got := headerString(123); got != "123" {
		t.Fatalf("default header string = %q", got)
	}
}
