package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lkal "github.com/Kaikei-e/LKAL"
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

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	validFile := writeTemp(t, dir, "valid.lkal", "@lkal \"0.1\"\na = 1\n")
	invalidFile1 := writeTemp(t, dir, "invalid1.lkal", "@lkal \"0.1\"\na = b\n")
	invalidFile2 := writeTemp(t, dir, "invalid2.lkal", "@lkal \"0.1\"\nx = y\n")

	err1 := invalidFile1 + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
	err2 := invalidFile2 + ":2:5: L102 bare word in value position\n  hint: write \"y\" for a string or .y for an enum\n"

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "valid file",
			args:       []string{"check", validFile},
			wantCode:   0,
			wantStdout: "",
			wantStderr: "",
		},
		{
			name:       "file with bare word",
			args:       []string{"check", invalidFile1},
			wantCode:   1,
			wantStdout: "",
			wantStderr: err1,
		},
		{
			name:       "two invalid files",
			args:       []string{"check", invalidFile1, invalidFile2},
			wantCode:   1,
			wantStdout: "",
			wantStderr: err1 + err2,
		},
		{
			name:       "one valid and one invalid",
			args:       []string{"check", validFile, invalidFile2},
			wantCode:   1,
			wantStdout: "",
			wantStderr: err2,
		},
		{
			name:       "one invalid and one valid",
			args:       []string{"check", invalidFile1, validFile},
			wantCode:   1,
			wantStdout: "",
			wantStderr: err1,
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

func TestCheckJSON(t *testing.T) {
	dir := t.TempDir()
	validFile := writeTemp(t, dir, "valid.lkal", "@lkal \"0.1\"\na = 1\n")
	invalidFile1 := writeTemp(t, dir, "invalid1.lkal", "@lkal \"0.1\"\na = b\n")
	invalidFile2 := writeTemp(t, dir, "invalid2.lkal", "@lkal \"0.1\"\nx = y\n")

	jsonErr1 := fmt.Sprintf(`[{"file":%q,"line":2,"col":5,"code":"L102","message":"bare word in value position","hint":"write \"b\" for a string or .b for an enum"}]`+"\n", invalidFile1)
	jsonErrBoth := fmt.Sprintf(`[{"file":%q,"line":2,"col":5,"code":"L102","message":"bare word in value position","hint":"write \"b\" for a string or .b for an enum"},{"file":%q,"line":2,"col":5,"code":"L102","message":"bare word in value position","hint":"write \"y\" for a string or .y for an enum"}]`+"\n", invalidFile1, invalidFile2)

	type errEntry struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Col     int    `json:"col"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Hint    string `json:"hint"`
	}

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
		parseJSON  bool
	}{
		{
			name:       "invalid file double dash",
			args:       []string{"check", "--json", invalidFile1},
			wantCode:   1,
			wantStdout: "",
			wantStderr: jsonErr1,
			parseJSON:  true,
		},
		{
			name:       "invalid file single dash",
			args:       []string{"check", "-json", invalidFile1},
			wantCode:   1,
			wantStdout: "",
			wantStderr: jsonErr1,
			parseJSON:  true,
		},
		{
			name:       "valid file",
			args:       []string{"check", "--json", validFile},
			wantCode:   0,
			wantStdout: "",
			wantStderr: "",
			parseJSON:  false,
		},
		{
			name:       "two invalid files",
			args:       []string{"check", "--json", invalidFile1, invalidFile2},
			wantCode:   1,
			wantStdout: "",
			wantStderr: jsonErrBoth,
			parseJSON:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := execRun(t, tc.args)
			if code != tc.wantCode || stdout != tc.wantStdout || stderr != tc.wantStderr {
				t.Errorf("run(%v):\nexit code: got %d, want %d\nstdout: got %q, want %q\nstderr: got %q, want %q",
					tc.args, code, tc.wantCode, stdout, tc.wantStdout, stderr, tc.wantStderr)
			}
			if tc.parseJSON && stderr != "" {
				var parsed []errEntry
				if err := json.Unmarshal([]byte(stderr), &parsed); err != nil {
					t.Errorf("stderr is not valid JSON: %v\ncontent: %q", err, stderr)
				}
			}
		})
	}
}

func TestFmt(t *testing.T) {
	t.Run("single non-canonical to stdout", func(t *testing.T) {
		dir := t.TempDir()
		orig := "@lkal \"0.1\"\na = 1, b = [1,2]\n"
		p := writeTemp(t, dir, "doc.lkal", orig)

		code, stdout, stderr := execRun(t, []string{"fmt", p})
		wantStdout := "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != orig {
			t.Errorf("file modified: got %q, want %q", string(data), orig)
		}
	})

	t.Run("fmt with json flag on success", func(t *testing.T) {
		dir := t.TempDir()
		orig := "@lkal \"0.1\"\na = 1, b = [1,2]\n"
		p := writeTemp(t, dir, "doc.lkal", orig)

		code, stdout, stderr := execRun(t, []string{"fmt", "--json", p})
		wantStdout := "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt --json:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
	})

	t.Run("multiple files in argument order", func(t *testing.T) {
		dir := t.TempDir()
		p1 := writeTemp(t, dir, "doc1.lkal", "@lkal \"0.1\"\na = 1\n")
		p2 := writeTemp(t, dir, "doc2.lkal", "@lkal \"0.1\"\nb = 2\n")

		code, stdout, stderr := execRun(t, []string{"fmt", p1, p2})
		wantStdout := "@lkal \"0.1\"\n\na = 1\n@lkal \"0.1\"\n\nb = 2\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt multiple:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
	})

	t.Run("l flag lists only non-canonical", func(t *testing.T) {
		dir := t.TempDir()
		p1 := writeTemp(t, dir, "dirty.lkal", "@lkal \"0.1\"\na = 1, b = 2\n")
		p2 := writeTemp(t, dir, "clean.lkal", "@lkal \"0.1\"\n\nc = 3\n")

		code, stdout, stderr := execRun(t, []string{"fmt", "-l", p1, p2})
		wantStdout := p1 + "\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt -l:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
	})

	t.Run("l flag all canonical", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "clean.lkal", "@lkal \"0.1\"\n\na = 1\n")

		code, stdout, stderr := execRun(t, []string{"fmt", "-l", p})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("fmt -l clean:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
	})

	t.Run("w flag rewrites non-canonical and preserves mode", func(t *testing.T) {
		dir := t.TempDir()
		orig := "@lkal \"0.1\"\na = 1, b = [1,2]\n"
		p := writeTempMode(t, dir, "doc.lkal", orig, 0640)

		code, stdout, stderr := execRun(t, []string{"fmt", "-w", p})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("fmt -w:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		wantData := "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
		if err != nil || string(data) != wantData {
			t.Errorf("file data: got %q, want %q", string(data), wantData)
		}
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if fi.Mode().Perm() != 0640 {
			t.Errorf("mode: got %o, want 0640", fi.Mode().Perm())
		}
	})

	t.Run("w flag leaves canonical untouched", func(t *testing.T) {
		dir := t.TempDir()
		orig := "@lkal \"0.1\"\n\na = 1\n"
		p := writeTempMode(t, dir, "doc.lkal", orig, 0644)

		code, stdout, stderr := execRun(t, []string{"fmt", "-w", p})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("fmt -w canonical:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != orig {
			t.Errorf("file data: got %q, want %q", string(data), orig)
		}
	})

	t.Run("l and w flags together", func(t *testing.T) {
		dir := t.TempDir()
		p1 := writeTemp(t, dir, "dirty.lkal", "@lkal \"0.1\"\na = 1, b = 2\n")
		p2 := writeTemp(t, dir, "clean.lkal", "@lkal \"0.1\"\n\nc = 3\n")

		code, stdout, stderr := execRun(t, []string{"fmt", "-l", "-w", p1, p2})
		wantStdout := p1 + "\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt -l -w:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
		data1, err := os.ReadFile(p1)
		wantData1 := "@lkal \"0.1\"\n\na = 1\nb = 2\n"
		if err != nil || string(data1) != wantData1 {
			t.Errorf("dirty file rewritten: got %q, want %q", string(data1), wantData1)
		}
	})

	t.Run("plain fmt with mix of valid and invalid files", func(t *testing.T) {
		dir := t.TempDir()
		pGood := writeTemp(t, dir, "good.lkal", "@lkal \"0.1\"\na = 1, b = [1,2]\n")
		pBad := writeTemp(t, dir, "bad.lkal", "@lkal \"0.1\"\na = b\n")

		code, stdout, stderr := execRun(t, []string{"fmt", pGood, pBad})
		wantStdout := "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
		wantStderr := pBad + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
		if code != 1 || stdout != wantStdout || stderr != wantStderr {
			t.Errorf("fmt good bad:\nexit code: got %d, want 1\nstdout: got %q, want %q\nstderr: got %q, want %q",
				code, stdout, wantStdout, stderr, wantStderr)
		}
	})

	t.Run("l flag with mix of valid and invalid files", func(t *testing.T) {
		dir := t.TempDir()
		pGood := writeTemp(t, dir, "good.lkal", "@lkal \"0.1\"\na = 1, b = [1,2]\n")
		pBad := writeTemp(t, dir, "bad.lkal", "@lkal \"0.1\"\na = b\n")

		code, stdout, stderr := execRun(t, []string{"fmt", "-l", pGood, pBad})
		wantStdout := pGood + "\n"
		wantStderr := pBad + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
		if code != 1 || stdout != wantStdout || stderr != wantStderr {
			t.Errorf("fmt -l good bad:\nexit code: got %d, want 1\nstdout: got %q, want %q\nstderr: got %q, want %q",
				code, stdout, wantStdout, stderr, wantStderr)
		}
	})

	t.Run("w flag isolation with error", func(t *testing.T) {
		dir := t.TempDir()
		badOrig := "@lkal \"0.1\"\na = b\n"
		goodOrig := "@lkal \"0.1\"\nx = 1, y = 2\n"
		pBad := writeTemp(t, dir, "bad.lkal", badOrig)
		pGood := writeTemp(t, dir, "good.lkal", goodOrig)

		code, stdout, stderr := execRun(t, []string{"fmt", "-w", pBad, pGood})
		wantStderr := pBad + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("fmt -w with error:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		badData, err := os.ReadFile(pBad)
		if err != nil || string(badData) != badOrig {
			t.Errorf("bad file was modified: got %q, want %q", string(badData), badOrig)
		}
		goodData, err := os.ReadFile(pGood)
		wantGood := "@lkal \"0.1\"\n\nx = 1\ny = 2\n"
		if err != nil || string(goodData) != wantGood {
			t.Errorf("good file was not rewritten: got %q, want %q", string(goodData), wantGood)
		}
	})

	t.Run("w flag with json flag and error", func(t *testing.T) {
		dir := t.TempDir()
		badOrig := "@lkal \"0.1\"\na = b\n"
		goodOrig := "@lkal \"0.1\"\nx = 1, y = 2\n"
		pBad := writeTemp(t, dir, "bad.lkal", badOrig)
		pGood := writeTemp(t, dir, "good.lkal", goodOrig)

		code, stdout, stderr := execRun(t, []string{"fmt", "-w", "--json", pBad, pGood})
		wantStderr := fmt.Sprintf(`[{"file":%q,"line":2,"col":5,"code":"L102","message":"bare word in value position","hint":"write \"b\" for a string or .b for an enum"}]`+"\n", pBad)
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("fmt -w --json with error:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		goodData, err := os.ReadFile(pGood)
		wantGood := "@lkal \"0.1\"\n\nx = 1\ny = 2\n"
		if err != nil || string(goodData) != wantGood {
			t.Errorf("good file was not rewritten: got %q, want %q", string(goodData), wantGood)
		}
	})
}

func TestJSON(t *testing.T) {
	dir := t.TempDir()
	simpleDoc := writeTemp(t, dir, "simple.lkal", "@lkal \"0.1\"\na = 1\n")
	tagDoc := writeTemp(t, dir, "tags.lkal", "@lkal \"0.1\"\ne = .active\nq = 512Mi\n")
	badDoc := writeTemp(t, dir, "bad.lkal", "@lkal \"0.1\"\na = b\n")

	wantSimpleLKAL := "{\n  \"$lkal\": \"0.1\",\n  \"a\": 1\n}\n"
	wantSimplePlain := "{\n  \"a\": 1\n}\n"
	wantTagLKAL := "{\n  \"$lkal\": \"0.1\",\n  \"e\": {\n    \"$enum\": \"active\"\n  },\n  \"q\": {\n    \"$qty\": \"512Mi\"\n  }\n}\n"
	wantTagPlain := "{\n  \"e\": \"active\",\n  \"q\": \"512Mi\"\n}\n"
	wantBadErr := badDoc + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
	wantBadJSONErr := fmt.Sprintf(`[{"file":%q,"line":2,"col":5,"code":"L102","message":"bare word in value position","hint":"write \"b\" for a string or .b for an enum"}]`+"\n", badDoc)

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "LKAL-JSON simple",
			args:       []string{"json", simpleDoc},
			wantCode:   0,
			wantStdout: wantSimpleLKAL,
			wantStderr: "",
		},
		{
			name:       "LKAL-JSON tags",
			args:       []string{"json", tagDoc},
			wantCode:   0,
			wantStdout: wantTagLKAL,
			wantStderr: "",
		},
		{
			name:       "plain JSON double dash",
			args:       []string{"json", "--plain", simpleDoc},
			wantCode:   0,
			wantStdout: wantSimplePlain,
			wantStderr: "",
		},
		{
			name:       "plain JSON single dash tags",
			args:       []string{"json", "-plain", tagDoc},
			wantCode:   0,
			wantStdout: wantTagPlain,
			wantStderr: "",
		},
		{
			name:       "file with parse error",
			args:       []string{"json", badDoc},
			wantCode:   1,
			wantStdout: "",
			wantStderr: wantBadErr,
		},
		{
			name:       "file with parse error and --json",
			args:       []string{"json", "--json", badDoc},
			wantCode:   1,
			wantStdout: "",
			wantStderr: wantBadJSONErr,
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

func TestFromJSON(t *testing.T) {
	dir := t.TempDir()
	goodJSON := writeTemp(t, dir, "doc.json", "{\n  \"$lkal\": \"0.1\",\n  \"a\": 1\n}\n")
	nullJSON := writeTemp(t, dir, "null.json", "{\"$lkal\": \"0.1\", \"a\": null}")
	noVersionJSON := writeTemp(t, dir, "no_version.json", "{\"a\": null}")

	wantLKAL := "@lkal \"0.1\"\n\na = 1\n"
	wantNullErr := nullJSON + ":1:23: L302 null value\n  hint: omit the key instead of writing null\n"
	wantNullJSONErr := fmt.Sprintf(`[{"file":%q,"line":1,"col":23,"code":"L302","message":"null value","hint":"omit the key instead of writing null"}]`+"\n", nullJSON)

	wantNoVersionErr := noVersionJSON + ":1:2: L105 missing, misplaced or repeated version declaration\n  hint: start the top-level object with \"$lkal\": \"0.1\"\n" +
		noVersionJSON + ":1:7: L302 null value\n  hint: omit the key instead of writing null\n"
	wantNoVersionJSONErr := fmt.Sprintf(`[{"file":%q,"line":1,"col":2,"code":"L105","message":"missing, misplaced or repeated version declaration","hint":"start the top-level object with \"$lkal\": \"0.1\""},{"file":%q,"line":1,"col":7,"code":"L302","message":"null value","hint":"omit the key instead of writing null"}]`+"\n", noVersionJSON, noVersionJSON)

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "valid small JSON",
			args:       []string{"from-json", goodJSON},
			wantCode:   0,
			wantStdout: wantLKAL,
			wantStderr: "",
		},
		{
			name:       "JSON with null error",
			args:       []string{"from-json", nullJSON},
			wantCode:   1,
			wantStdout: "",
			wantStderr: wantNullErr,
		},
		{
			name:       "JSON with null error and --json",
			args:       []string{"from-json", "--json", nullJSON},
			wantCode:   1,
			wantStdout: "",
			wantStderr: wantNullJSONErr,
		},
		{
			name:       "JSON with missing version and null error",
			args:       []string{"from-json", noVersionJSON},
			wantCode:   1,
			wantStdout: "",
			wantStderr: wantNoVersionErr,
		},
		{
			name:       "JSON with missing version and null error and --json",
			args:       []string{"from-json", "--json", noVersionJSON},
			wantCode:   1,
			wantStdout: "",
			wantStderr: wantNoVersionJSONErr,
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

func TestSet(t *testing.T) {
	t.Run("spec command updates image and keeps comments", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/container/app/image", `"ghcr.io/acme/web:1.5.0"`})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("set:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		wantContent := strings.Replace(specExampleDoc, "1.4.2", "1.5.0", 1)
		if string(data) != wantContent {
			t.Errorf("file content mismatch:\ngot:\n%s\nwant:\n%s", string(data), wantContent)
		}
		f, parseErr := lkal.Parse(p, data)
		if parseErr != nil {
			t.Errorf("rewritten file failed to parse: %v", parseErr)
		} else if f.Version() != "0.1" {
			t.Errorf("version: got %q, want \"0.1\"", f.Version())
		}
	})

	t.Run("set adds element under collection", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/container/app/port/admin", "{ number = 8081, protocol = .tcp }"})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("set add element:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "admin") || !strings.Contains(content, "8081") {
			t.Errorf("new element missing from file: %s", content)
		}
		if _, parseErr := lkal.Parse(p, data); parseErr != nil {
			t.Errorf("rewritten file failed to parse: %v", parseErr)
		}
	})

	t.Run("bad value exits 1 and leaves file unchanged", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/replicas", "tcp"})
		wantStderr := "<value>:1:1: L102 bare word in value position\n  hint: write \"tcp\" for a string or .tcp for an enum\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set bad value:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("bad value with json flag has file <value>", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", "--json", p, "service/web/replicas", "tcp"})
		wantStderr := "[{\"file\":\"<value>\",\"line\":1,\"col\":1,\"code\":\"L102\",\"message\":\"bare word in value position\",\"hint\":\"write \\\"tcp\\\" for a string or .tcp for an enum\"}]\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set bad value --json:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("non-record value for collection element", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/container/app/port/http", "1"})
		wantStderr := p + ": L403 invalid value\n  hint: use a record for a collection element\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set non-record on collection element:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("malformed path", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "bad//path", "\"val\""})
		wantStderr := p + ": L401 malformed path\n  hint: separate segments with / and quote segments that contain other characters\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set malformed path:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("missing parent exits 1 and leaves file unchanged", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/missing/image", "\"foo\""})
		wantStderr := p + ": L402 path not found\n  hint: check the path at segment \"missing\"\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set missing parent:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("write failure on read-only file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("skipping write failure test as root")
		}
		dir := t.TempDir()
		p := writeTempMode(t, dir, "ro.lkal", specExampleDoc, 0400)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/container/app/image", `"ghcr.io/acme/web:1.5.0"`})
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "lkal: ") {
			t.Errorf("set on read-only file:\nexit code: got %d, want 2\nstdout: got %q, want \"\"\nstderr: got %q, want prefix \"lkal: \"",
				code, stdout, stderr)
		}
	})
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
