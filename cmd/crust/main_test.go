package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "version",
			args:       []string{"--version"},
			wantCode:   0,
			wantStdout: "crust " + version,
		},
		{
			name:       "help long flag",
			args:       []string{"--help"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "help short flag",
			args:       []string{"-h"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "no args prints usage and fails",
			args:       []string{},
			wantCode:   1,
			wantStderr: "Usage:",
		},
		{
			name:       "run stub",
			args:       []string{"run", "day01.crust"},
			wantCode:   1,
			wantStderr: "not in the oven yet",
		},
		{
			name:       "repl stub",
			args:       []string{"repl"},
			wantCode:   1,
			wantStderr: "not in the oven yet",
		},
		{
			name:       "unknown command",
			args:       []string{"bogus"},
			wantCode:   1,
			wantStderr: `unknown command "bogus"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want substring %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
