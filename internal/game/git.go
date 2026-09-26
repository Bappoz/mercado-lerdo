package game

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// The Git app works on the player's workspace repository with a fixed set of
// commands: no shell, the player's hooks off, paths only from `git status`.

// FileChange is one entry of `git status`.
type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"` // modified | added | deleted | untracked | renamed
}

// Commit is one entry of the log.
type Commit struct {
	Hash    string   `json:"hash"`
	Short   string   `json:"short"`
	Author  string   `json:"author"`
	Date    string   `json:"date"` // RFC 3339
	Subject string   `json:"subject"`
	Refs    []string `json:"refs,omitempty"`
}

// maxDiff bounds what a diff sends to the view.
const maxDiff = 256 << 10

func gitCmd(ctx context.Context, dir string, args ...string) *exec.Cmd {
	full := append([]string{
		"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false",
		"-c", "user.name=Você", "-c", "user.email=voce@nexora.local", "-c", "core.quotePath=false",
	}, args...)
	cmd := exec.CommandContext(ctx, "git", full...) //nolint:gosec // G204: fixed binary; arguments built here
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	return cmd
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	var out, errb bytes.Buffer
	cmd := gitCmd(ctx, dir, args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	_, err := gitOutput(ctx, dir, args...)
	return err
}

// gitCommitAll commits everything (an empty commit when nothing changed).
func gitCommitAll(ctx context.Context, dir, message string) error {
	if err := runGit(ctx, dir, "add", "-A"); err != nil {
		return err
	}
	return runGit(ctx, dir, "commit", "-q", "--allow-empty", "-m", message)
}

// GitStatus lists what changed in the workspace since the last commit.
func (g *Game) GitStatus(ctx context.Context) ([]FileChange, error) {
	out, err := gitOutput(ctx, g.Session().Workspace, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var changes []FileChange
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		x, y, path := f[0], f[1], f[3:]
		c := FileChange{Path: path, Status: "modified"}
		switch {
		case x == '?':
			c.Status = "untracked"
		case x == 'A':
			c.Status = "added"
		case x == 'D' || y == 'D':
			c.Status = "deleted"
		case x == 'R':
			c.Status = "renamed"
			i++ // -z puts the source path in the next field
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// GitDiff shows the changes of path (one of GitStatus's, or "" for all)
// against the last commit, or against the scenario's start when sinceStart.
func (g *Game) GitDiff(ctx context.Context, path string, sinceStart bool) (string, error) {
	ws := g.Session().Workspace
	base := "HEAD"
	if sinceStart {
		base = "sesim/scenario-start/" + filepath.Base(g.Pack().Dir)
	}
	// Fixed prefixes: the player's ~/.gitconfig (mnemonicPrefix, noprefix) must not change the format.
	args := []string{"diff", "--no-color", "--no-ext-diff", "--src-prefix=a/", "--dst-prefix=b/", base}
	changes, err := g.GitStatus(ctx)
	if err != nil {
		return "", err
	}
	if path != "" {
		i := slices.IndexFunc(changes, func(c FileChange) bool { return c.Path == path })
		if i < 0 && !sinceStart {
			return "", fmt.Errorf("%s não mudou desde o último commit", path)
		}
		if i >= 0 && changes[i].Status == "untracked" {
			return untrackedDiff(ws, path)
		}
		args = append(args, "--", path)
	}
	out, err := gitOutput(ctx, ws, args...)
	if err != nil {
		return "", err
	}
	// git diff leaves out new files, the ones a player most often adds.
	for _, c := range changes {
		if path != "" || c.Status != "untracked" || len(out) > maxDiff {
			continue
		}
		d, err := untrackedDiff(ws, c.Path)
		if err != nil {
			return "", err
		}
		out += "diff --git a/" + c.Path + " b/" + c.Path + "\n" + d
	}
	if len(out) > maxDiff {
		out = out[:maxDiff] + "\n… (diff cortado)\n"
	}
	return out, nil
}

// untrackedDiff renders a new file as a diff that adds every line.
func untrackedDiff(ws, path string) (string, error) {
	full := filepath.Join(ws, filepath.Clean("/"+path)) // path came from git status; stay inside ws anyway
	// A link is shown as its target, like git does, never followed out of the workspace.
	if fi, err := os.Lstat(full); err != nil {
		return "", err
	} else if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("--- /dev/null\n+++ b/%s\n+%s\n\\ link simbólico\n", path, target), nil
	}
	b, err := os.ReadFile(full) //nolint:gosec // G304: a regular file git status listed, inside the workspace
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return fmt.Sprintf("arquivo novo (binário): %s\n", path), nil
	}
	if len(b) > maxDiff {
		b = b[:maxDiff]
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- /dev/null\n+++ b/%s\n", path)
	for line := range strings.Lines(string(b)) {
		sb.WriteString("+" + line)
	}
	return sb.String(), nil
}

// GitLog lists the latest commits of the workspace.
func (g *Game) GitLog(ctx context.Context, n int) ([]Commit, error) {
	out, err := gitOutput(ctx, g.Session().Workspace, "log", fmt.Sprintf("-n%d", n),
		"--date=iso-strict", "--format=%H%x1f%h%x1f%an%x1f%ad%x1f%s%x1f%D%x1e")
	if err != nil {
		return nil, err
	}
	var log []Commit
	for rec := range strings.SplitSeq(out, "\x1e") {
		f := strings.Split(strings.TrimSpace(rec), "\x1f")
		if len(f) < 6 {
			continue
		}
		c := Commit{Hash: f[0], Short: f[1], Author: f[2], Date: f[3], Subject: f[4]}
		for r := range strings.SplitSeq(f[5], ", ") {
			if r = strings.TrimPrefix(strings.TrimSpace(r), "tag: "); r != "" && r != "HEAD" && !strings.HasPrefix(r, "HEAD ->") {
				c.Refs = append(c.Refs, r)
			}
		}
		log = append(log, c)
	}
	return log, nil
}

// ErrNothingToCommit means the workspace has no changes.
var ErrNothingToCommit = errors.New("nada para commitar: o workspace está igual ao último commit")

// GitCommit commits every change of the workspace with message.
func (g *Game) GitCommit(ctx context.Context, message string) (Commit, error) {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 500 || strings.ContainsRune(message, 0) {
		return Commit{}, errors.New("a mensagem precisa ter de 1 a 500 caracteres")
	}
	changes, err := g.GitStatus(ctx)
	if err != nil {
		return Commit{}, err
	}
	if len(changes) == 0 {
		return Commit{}, ErrNothingToCommit
	}
	ws := g.Session().Workspace
	if err := runGit(ctx, ws, "add", "-A"); err != nil {
		return Commit{}, err
	}
	if err := runGit(ctx, ws, "commit", "-q", "-m", message); err != nil {
		return Commit{}, err
	}
	g.Event("git_commit", map[string]any{"files": len(changes)})
	log, err := g.GitLog(ctx, 1)
	if err != nil || len(log) == 0 {
		return Commit{}, err
	}
	return log[0], nil
}
