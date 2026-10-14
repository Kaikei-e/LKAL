package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	lkal "github.com/Kaikei-e/LKAL"
)

func TestGet(t *testing.T) {
	dir := t.TempDir()
	specFile := writeTemp(t, dir, "spec.lkal", specExampleDoc)
	recordFile := writeTemp(t, dir, "record.lkal", "@lkal \"0.1\"\nr { x = 1, y = 2 }\n")

	wantPortCollection := "\"http\" {\n  number = 8080\n  protocol = .tcp\n}\n\"metrics\" {\n  number = 9090\n  protocol = .tcp\n}\n"

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "spec example int",
			args:       []string{"get", specFile, "service/web/container/app/port/http/number"},
			wantCode:   0,
			wantStdout: "8080\n",
			wantStderr: "",
		},
		{
			name:       "spec example int with json flag on success",
			args:       []string{"get", "--json", specFile, "service/web/container/app/port/http/number"},
			wantCode:   0,
			wantStdout: "8080\n",
			wantStderr: "",
		},
		{
			name:       "spec example string",
			args:       []string{"get", specFile, "service/web/container/app/image"},
			wantCode:   0,
			wantStdout: "\"ghcr.io/acme/web:1.4.2\"\n",
			wantStderr: "",
		},
		{
			name:       "quoted segment string",
			args:       []string{"get", specFile, `service/web/labels/"app.kubernetes.io/name"`},
			wantCode:   0,
			wantStdout: "\"web\"\n",
			wantStderr: "",
		},
		{
			name:       "spec example enum",
			args:       []string{"get", specFile, "service/web/container/app/port/http/protocol"},
			wantCode:   0,
			wantStdout: ".tcp\n",
			wantStderr: "",
		},
		{
			name:       "spec example list",
			args:       []string{"get", specFile, "service/web/container/app/args"},
			wantCode:   0,
			wantStdout: "[\"--port\", \"8080\"]\n",
			wantStderr: "",
		},
		{
			name:       "spec example raw string",
			args:       []string{"get", specFile, "service/web/startup"},
			wantCode:   0,
			wantStdout: "\\\\#!/bin/sh\n\\\\exec /app/web --port 8080\n",
			wantStderr: "",
		},
		{
			name:       "spec example record",
			args:       []string{"get", specFile, "service/web/container/app/port/http"},
			wantCode:   0,
			wantStdout: "{\n  number = 8080\n  protocol = .tcp\n}\n",
			wantStderr: "",
		},
		{
			name:       "collection key layout",
			args:       []string{"get", specFile, "service/web/container/app/port"},
			wantCode:   0,
			wantStdout: wantPortCollection,
			wantStderr: "",
		},
		{
			name:       "simple record",
			args:       []string{"get", recordFile, "r"},
			wantCode:   0,
			wantStdout: "{\n  x = 1\n  y = 2\n}\n",
			wantStderr: "",
		},
		{
			name:       "missing path",
			args:       []string{"get", specFile, "service/web/container/app/port/api"},
			wantCode:   1,
			wantStdout: "",
			wantStderr: specFile + ": L402 path not found\n  hint: check the path at segment \"api\"\n",
		},
		{
			name:       "missing path with --json",
			args:       []string{"get", "--json", specFile, "service/web/container/app/port/api"},
			wantCode:   1,
			wantStdout: "",
			wantStderr: fmt.Sprintf(`[{"file":%q,"line":0,"col":0,"code":"L402","message":"path not found","hint":"check the path at segment \"api\""}]`+"\n", specFile),
		},
		{
			name:       "malformed path",
			args:       []string{"get", specFile, "bad//path"},
			wantCode:   1,
			wantStdout: "",
			wantStderr: specFile + ": L401 malformed path\n  hint: separate segments with / and quote segments that contain other characters\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := execRun(t, tc.args)
			if code != tc.wantCode || stdout != tc.wantStdout || stderr != tc.wantStderr {
				t.Errorf("run(%v):\nexit code: got %d, want %d\nstdout: got %q, want %q\nstderr: got %q, want %q",
					tc.args, code, tc.wantCode, stdout, tc.wantStdout, stderr, tc.wantStderr)
			}
		})
	}
}

func TestDel(t *testing.T) {
	t.Run("remove entry rewrites file canonically", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"del", p, "service/web/replicas"})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("del:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		wantContent := strings.Replace(specExampleDoc, "  replicas = 3\n", "", 1)
		if string(data) != wantContent {
			t.Errorf("file content mismatch:\ngot:\n%s\nwant:\n%s", string(data), wantContent)
		}
		if _, parseErr := lkal.Parse(p, data); parseErr != nil {
			t.Errorf("rewritten file failed to parse: %v", parseErr)
		}
	})

	t.Run("remove entry with json flag on success", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"del", "--json", p, "service/web/replicas"})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("del --json:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		wantContent := strings.Replace(specExampleDoc, "  replicas = 3\n", "", 1)
		if string(data) != wantContent {
			t.Errorf("file content mismatch:\ngot:\n%s\nwant:\n%s", string(data), wantContent)
		}
	})

	t.Run("remove collection element", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"del", p, "service/web/container/app/port/metrics"})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("del element:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		content := string(data)
		if strings.Contains(content, "metrics") {
			t.Errorf("deleted collection element still in file: %s", content)
		}
		if !strings.Contains(content, "http") {
			t.Errorf("remaining collection element missing from file: %s", content)
		}
		if _, parseErr := lkal.Parse(p, data); parseErr != nil {
			t.Errorf("rewritten file failed to parse: %v", parseErr)
		}
	})

	t.Run("missing path exits 1 and leaves file unchanged", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"del", p, "service/web/nonexistent"})
		wantStderr := p + ": L402 path not found\n  hint: check the path at segment \"nonexistent\"\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("del missing path:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("missing path with json flag", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"del", "--json", p, "service/web/nonexistent"})
		wantStderr := fmt.Sprintf(`[{"file":%q,"line":0,"col":0,"code":"L402","message":"path not found","hint":"check the path at segment \"nonexistent\""}]`+"\n", p)
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("del missing path --json:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})
}
