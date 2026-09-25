package store

import "testing"

func TestEscapeLike(t *testing.T) {
	tests := []struct{ in, want string }{
		{"lamp", "lamp"},
		{"100%", `100\%`},
		{"a_b", `a\_b`},
		{`back\slash`, `back\\slash`},
	}
	for _, tt := range tests {
		if got := escapeLike(tt.in); got != tt.want {
			t.Errorf("escapeLike(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
