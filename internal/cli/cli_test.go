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
		{name: "no arguments", args: nil, wantCode: 2, wantStderr: "Uso:"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "sesim "},
		{name: "--version", args: []string{"--version"}, wantCode: 0, wantStdout: "sesim "},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "Para jogar:"},
		{name: "load-summary bad input", args: []string{"load-summary"}, stdin: "nope", wantCode: 1, wantStderr: "decode k6 summary"},
		{name: "dev without command", args: []string{"dev"}, wantCode: 2, wantStderr: "container-stats"},
		{name: "dev unknown", args: []string{"dev", "fly"}, wantCode: 2, wantStderr: `comando desconhecido "fly"`},
		{name: "unknown", args: []string{"fly"}, wantCode: 2, wantStderr: `comando desconhecido "fly"`},
		{name: "tabby step is a number", args: []string{"tabby", "passo", "dez"}, wantCode: 2, wantStderr: "uso: sesim tabby passo [número]"},
		{name: "tabby unknown", args: []string{"tabby", "fly"}, wantCode: 2, wantStderr: "passo [número]"},
		{name: "clock unknown", args: []string{"clock", "fly"}, wantCode: 2, wantStderr: "uso: sesim clock [pause | resume]"},
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
