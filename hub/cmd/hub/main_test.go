package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCommands(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "Dislackso hub "},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "Usage:"},
		{name: "run help", args: []string{"run", "-h"}, wantCode: 0, wantStderr: "-http-addr"},
		{name: "flags without a command mean run", args: []string{"-h"}, wantCode: 0, wantStderr: "-data-dir"},
		{name: "unknown command", args: []string{"nope"}, wantCode: 2, wantStderr: `unknown command "nope"`},
		{name: "invalid config", args: []string{"run", "--log-format", "xml"}, wantCode: 2, wantStderr: "log.format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
