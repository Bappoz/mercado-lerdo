package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("cavalo correto")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{"senha certa", "cavalo correto", true},
		{"senha errada", "grampo bateria", false},
		{"senha vazia", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyPassword(tt.password, hash)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("VerifyPassword(%q) = %v, esperado %v", tt.password, got, tt.want)
			}
		})
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, encoded := range []string{"", "plain", "bcrypt$1$a$b", "pbkdf2-sha256$x$a$b", "pbkdf2-sha256$10$!!$b"} {
		if _, err := VerifyPassword("x", encoded); err == nil {
			t.Errorf("VerifyPassword(_, %q) não devolveu erro", encoded)
		}
	}
}

func TestNewTokenUnique(t *testing.T) {
	a, _ := NewToken()
	b, _ := NewToken()
	if a == b || len(a) < 40 {
		t.Fatalf("tokens repetidos ou curtos demais: %q %q", a, b)
	}
}
