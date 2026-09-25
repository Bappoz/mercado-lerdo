package buildinfo

import "testing"

func TestInfoString(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "short commit",
			info: Info{Version: "v0.1.0", Commit: "abc123", Go: "go1.27.1", Platform: "linux/amd64"},
			want: "sesim v0.1.0 (abc123, go1.27.1, linux/amd64)",
		},
		{
			name: "long commit is truncated and marked dirty",
			info: Info{Version: "dev", Commit: "0123456789abcdef", Modified: true, Go: "go1.27.1", Platform: "darwin/arm64"},
			want: "sesim dev (0123456789ab-dirty, go1.27.1, darwin/arm64)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadDefaults(t *testing.T) {
	info := Read()
	if info.Version == "" || info.Go == "" || info.Platform == "" {
		t.Fatalf("Read() returned empty fields: %+v", info)
	}
}
