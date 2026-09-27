package docker

import (
	"bytes"
	"strings"
	"testing"
)

func TestCappedKeepsTheHeadOnly(t *testing.T) {
	var c capped
	chunk := bytes.Repeat([]byte("x"), 1<<20)
	for range 20 { // 20 MiB against a 16 MiB cap
		if n, err := c.Write(chunk); n != len(chunk) || err != nil {
			t.Fatalf("Write = %d, %v: a full buffer must not fail the copy", n, err)
		}
	}
	if c.b.Len() != maxOutput || !strings.HasSuffix(c.String(), "(output cut at 16 MiB)\n") {
		t.Errorf("kept %d bytes, suffix %q", c.b.Len(), c.String()[len(c.String())-30:])
	}
	var small capped
	_, _ = small.Write([]byte("ok\n"))
	if small.String() != "ok\n" {
		t.Errorf("small output = %q", small.String())
	}
}
