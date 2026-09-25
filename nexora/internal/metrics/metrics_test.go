package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestHistogramIsCumulative(t *testing.T) {
	m := New(nil)
	m.Observe("GET /search", 200, 3*time.Millisecond)   // bucket .005
	m.Observe("GET /search", 200, 300*time.Millisecond) // bucket .5
	m.Observe("GET /search", 500, 20*time.Second)       // only +Inf

	var b strings.Builder
	m.Render(&b)
	out := b.String()

	for _, want := range []string{
		`nexora_http_requests_total{route="GET /search",code="200"} 2`,
		`nexora_http_requests_total{route="GET /search",code="500"} 1`,
		`nexora_http_request_duration_seconds_bucket{route="GET /search",le="0.005"} 1`,
		`nexora_http_request_duration_seconds_bucket{route="GET /search",le="0.25"} 1`,
		`nexora_http_request_duration_seconds_bucket{route="GET /search",le="0.5"} 2`,
		`nexora_http_request_duration_seconds_bucket{route="GET /search",le="10"} 2`,
		`nexora_http_request_duration_seconds_bucket{route="GET /search",le="+Inf"} 3`,
		`nexora_http_request_duration_seconds_count{route="GET /search"} 3`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing line %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestEscape(t *testing.T) {
	if got := escape("a\"b\\c\nd"); got != `a\"b\\c\nd` {
		t.Errorf("escape = %q", got)
	}
}

func TestLabelWithQuoteIsEscapedOnce(t *testing.T) {
	m := New(nil)
	m.Observe(`GET /a"b`, 200, time.Millisecond)
	var b strings.Builder
	m.Render(&b)
	if want := `route="GET /a\"b",code="200"} 1`; !strings.Contains(b.String(), want) {
		t.Errorf("want %q in output:\n%s", want, b.String())
	}
}
