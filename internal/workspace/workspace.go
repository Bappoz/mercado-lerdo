// Package workspace manages copies of the player's app. Materialize builds a
// throwaway workspace from a base and an optional patch; the scenario harness
// uses it to evaluate reference solutions without touching the real one.
package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Materialize copies base into a new temporary directory and applies patch
// (a unified diff with a/ b/ prefixes) when it is not empty. The caller must
// call cleanup.
func Materialize(ctx context.Context, base, patch string) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "sesim-ws-")
	if err != nil {
		return "", nil, fmt.Errorf("workspace: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	if err := os.CopyFS(dir, os.DirFS(base)); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("workspace: copy %s: %w", base, err)
	}
	if patch == "" {
		return dir, cleanup, nil
	}
	abs, err := filepath.Abs(patch)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("workspace: %w", err)
	}
	// git apply works outside a repository as a strict patch tool: it refuses
	// to apply hunks that do not match exactly.
	cmd := exec.CommandContext(ctx, "git", "apply", "-p1", abs) //nolint:gosec // G204: fixed binary; abs is a patch path from a scenario pack
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("workspace: apply %s: %w: %s", patch, err, stderr.String())
	}
	return dir, cleanup, nil
}
