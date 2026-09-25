package stack

import (
	"os"
	"testing"
)

func TestRegistryRoundTrip(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	if rec, err := reg.Get("r-none"); err != nil || rec != nil {
		t.Fatalf("unknown run = %+v, %v", rec, err)
	}
	rec := Record{Run: "r-abc", PID: os.Getpid(), State: Ready, Secrets: NewSecrets()}
	if err := reg.Put(rec); err != nil {
		t.Fatal(err)
	}
	got, err := reg.Get("r-abc")
	if err != nil || got == nil || got.State != Ready || got.Secrets != rec.Secrets {
		t.Fatalf("got %+v, %v", got, err)
	}
	info, err := os.Stat(reg.path("r-abc"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("record permissions = %v, %v (holds secrets)", info.Mode().Perm(), err)
	}
	if err := reg.Delete("r-abc"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Delete("r-abc"); err != nil {
		t.Errorf("second delete: %v", err)
	}
}

func TestOrphan(t *testing.T) {
	const deadPID = 1 << 30 // far above any real pid
	tests := []struct {
		name string
		rec  *Record
		want bool
	}{
		{"unregistered", nil, true},
		{"creating, owner alive", &Record{State: Creating, PID: os.Getpid()}, false},
		{"creating, owner gone (killed mid-Up)", &Record{State: Creating, PID: deadPID}, true},
		{"ready, owner gone (normal: up exits)", &Record{State: Ready, PID: deadPID}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Orphan(tt.rec); got != tt.want {
				t.Errorf("Orphan = %v, want %v", got, tt.want)
			}
		})
	}
}
