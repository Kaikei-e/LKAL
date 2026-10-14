package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const specExampleDoc = `@lkal "0.1"

### フロントエンドのWebサーバー
service "web" {
  replicas = 3
  traffic-share = 0.25
  labels { "app.kubernetes.io/name" = "web", tier = "frontend" }

  container "app" {
    image = "ghcr.io/acme/web:1.4.2"
    args = ["--port", "8080"]
    memory = 512Mi
    cpu = 250m
    timeout = 30s
    port "http" { number = 8080, protocol = .tcp }
    port "metrics" { number = 9090, protocol = .tcp }
    env { LOG_LEVEL = "info" }
    secret = &secret/db-creds
  }

  ### 起動スクリプト。各行を \\ で始める
  startup =
    \\#!/bin/sh
    \\exec /app/web --port 8080
}
`

func execRun(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return p
}

func writeTempMode(t *testing.T, dir, name, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatalf("writeTempMode: %v", err)
	}
	return p
}

func hasUsageLine(stderr string) bool {
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, "usage: lkal") {
			return true
		}
	}
	return false
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no arguments", []string{}},
		{"unknown command", []string{"nope"}},
		{"unknown flag", []string{"check", "--nope", "f"}},
		{"fmt no file", []string{"fmt"}},
		{"fmt -l no file", []string{"fmt", "-l"}},
		{"fmt unknown flag", []string{"fmt", "--nope", "f"}},
		{"check no file", []string{"check"}},
		{"check unknown flag", []string{"check", "--nope", "f"}},
		{"json no file", []string{"json"}},
		{"json two files", []string{"json", "f1", "f2"}},
		{"json -plain two files", []string{"json", "-plain", "a", "b"}},
		{"json unknown flag", []string{"json", "--nope", "f"}},
		{"from-json no file", []string{"from-json"}},
		{"from-json two files", []string{"from-json", "f1", "f2"}},
		{"from-json unknown flag", []string{"from-json", "--nope", "f"}},
		{"get no args", []string{"get"}},
		{"get one arg", []string{"get", "f"}},
		{"get three args", []string{"get", "f", "p", "extra"}},
		{"get unknown flag", []string{"get", "--nope", "f", "p"}},
		{"set no args", []string{"set"}},
		{"set one arg", []string{"set", "f"}},
		{"set two args", []string{"set", "f", "p"}},
		{"set four args", []string{"set", "f", "p", "v", "extra"}},
		{"set unknown flag", []string{"set", "--nope", "f", "p", "v"}},
		{"del no args", []string{"del"}},
		{"del one arg", []string{"del", "f"}},
		{"del three args", []string{"del", "f", "p", "extra"}},
		{"del unknown flag", []string{"del", "--nope", "f", "p"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := execRun(t, tc.args)
			if code != 2 || stdout != "" || !hasUsageLine(stderr) {
				t.Errorf("run(%v):\nexit code: got %d, want 2\nstdout: got %q, want \"\"\nstderr: got %q, want a line starting with \"usage: lkal\"",
					tc.args, code, stdout, stderr)
			}
		})
	}
}

func TestIOErrors(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.lkal")

	tests := []struct {
		name string
		args []string
	}{
		{"check missing file", []string{"check", missing}},
		{"check missing file with json flag", []string{"check", "--json", missing}},
		{"get missing file", []string{"get", missing, "some/path"}},
		{"json missing file", []string{"json", missing}},
		{"from-json missing file", []string{"from-json", filepath.Join(dir, "missing.json")}},
		{"set missing file", []string{"set", missing, "some/path", "1"}},
		{"del missing file", []string{"del", missing, "some/path"}},
		{"fmt missing file", []string{"fmt", missing}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := execRun(t, tc.args)
			if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "lkal: ") {
				t.Errorf("run(%v):\nexit code: got %d, want 2\nstdout: got %q, want \"\"\nstderr: got %q, want prefix \"lkal: \"",
					tc.args, code, stdout, stderr)
			}
		})
	}
}

func TestFailedCommandStdout(t *testing.T) {
	dir := t.TempDir()
	goodFile := writeTemp(t, dir, "good.lkal", "@lkal \"0.1\"\na = 1\n")
	badFile := writeTemp(t, dir, "bad.lkal", "@lkal \"0.1\"\na = b\n")
	badJSON := writeTemp(t, dir, "bad.json", "{\"a\": null}")
	missingFile := filepath.Join(dir, "missing.lkal")

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"check parse error", []string{"check", badFile}, 1},
		{"fmt parse error single file", []string{"fmt", badFile}, 1},
		{"json parse error", []string{"json", badFile}, 1},
		{"from-json parse error", []string{"from-json", badJSON}, 1},
		{"get not found", []string{"get", goodFile, "missing"}, 1},
		{"set bad value", []string{"set", goodFile, "a", "tcp"}, 1},
		{"del not found", []string{"del", goodFile, "missing"}, 1},
		{"usage error", []string{"unknown-cmd"}, 2},
		{"io error", []string{"check", missingFile}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := execRun(t, tc.args)
			if code != tc.wantCode {
				t.Errorf("exit code: got %d, want %d", code, tc.wantCode)
			}
			if stdout != "" {
				t.Errorf("stdout must be empty on failure, got %q", stdout)
			}
			if stderr == "" {
				t.Errorf("stderr must not be empty on failure")
			}
		})
	}
}
