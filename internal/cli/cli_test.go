package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no arguments", args: nil, wantCode: 2, wantStderr: "Usage:"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "sesim "},
		{name: "--version", args: []string{"--version"}, wantCode: 0, wantStdout: "sesim "},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "Playing (output in PT-BR):"},
		{name: "load-summary bad input", args: []string{"load-summary"}, stdin: "nope", wantCode: 1, wantStderr: "decode k6 summary"},
		{name: "dev without command", args: []string{"dev"}, wantCode: 2, wantStderr: "container-stats"},
		{name: "dev unknown", args: []string{"dev", "fly"}, wantCode: 2, wantStderr: `unknown command "fly"`},
		{name: "unknown", args: []string{"fly"}, wantCode: 2, wantStderr: `unknown command "fly"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want contains %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want contains %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
