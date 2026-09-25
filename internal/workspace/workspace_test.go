package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialize(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "migrations"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	patch := filepath.Join(t.TempDir(), "p.patch")
	diff := `diff -ruN a/migrations/0002_x.sql b/migrations/0002_x.sql
--- a/migrations/0002_x.sql	1970-01-01 00:00:00.000000000 +0000
+++ b/migrations/0002_x.sql	2026-01-01 00:00:00.000000000 +0000
@@ -0,0 +1 @@
+SELECT 1;
`
	if err := os.WriteFile(patch, []byte(diff), 0o600); err != nil {
		t.Fatal(err)
	}

	dir, cleanup, err := Materialize(context.Background(), base, patch)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "migrations", "0002_x.sql"))
	if err != nil || strings.TrimSpace(string(got)) != "SELECT 1;" {
		t.Fatalf("patched file = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(base, "migrations", "0002_x.sql")); !os.IsNotExist(err) {
		t.Error("base was modified")
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("cleanup left the workspace behind")
	}

	// A patch whose context does not match the workspace (the base moved on)
	// must be refused, not half-applied.
	stale := `diff -ruN a/main.go b/main.go
--- a/main.go	1970-01-01 00:00:00.000000000 +0000
+++ b/main.go	2026-01-01 00:00:00.000000000 +0000
@@ -1 +1 @@
-package app
+package main2
`
	if err := os.WriteFile(patch, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Materialize(context.Background(), base, patch); err == nil || !strings.Contains(err.Error(), "apply") {
		t.Errorf("stale patch: err = %v, want an apply error", err)
	}
}
