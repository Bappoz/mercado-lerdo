package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{"right password", "correct horse", true},
		{"wrong password", "battery staple", false},
		{"empty password", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyPassword(tt.password, hash)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("VerifyPassword(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, encoded := range []string{"", "plain", "bcrypt$1$a$b", "pbkdf2-sha256$x$a$b", "pbkdf2-sha256$10$!!$b"} {
		if _, err := VerifyPassword("x", encoded); err == nil {
			t.Errorf("VerifyPassword(_, %q) returned nil error", encoded)
		}
	}
}

func TestNewTokenUnique(t *testing.T) {
	a, _ := NewToken()
	b, _ := NewToken()
	if a == b || len(a) < 40 {
		t.Fatalf("tokens not unique or too short: %q %q", a, b)
	}
}
