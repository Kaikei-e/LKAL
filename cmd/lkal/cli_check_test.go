package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestCheckJSONInvalidUTF8FileName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b\xffd.lkal")
	content := "@lkal \"0.1\"\na = b\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Skip("OS refused invalid UTF-8 filename:", err)
	}

	args := []string{"check", "--json", path}
	code, stdout, stderr := execRun(t, args)
	if code != 1 {
		t.Errorf("run(%v) exit code mismatch:\ngot %d, want 1\nstdout: got %q\nstderr: got %q", args, code, stdout, stderr)
	}

	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Errorf("run(%v) stderr is not exactly one non-empty line:\nexit code: got %d\nstdout: got %q\nstderr: got %q", args, code, stdout, stderr)
		return
	}

	type errEntry struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Col  int    `json:"col"`
		Code string `json:"code"`
	}

	var entries []errEntry
	if err := json.Unmarshal([]byte(lines[0]), &entries); err != nil {
		t.Errorf("run(%v) stderr JSON decode error: %v\nexit code: got %d\nstdout: got %q\nstderr: got %q", args, err, code, stdout, stderr)
		return
	}
	if len(entries) != 1 {
		t.Errorf("run(%v) entries count mismatch:\ngot %d, want 1\nexit code: got %d\nstdout: got %q\nstderr: got %q", args, len(entries), code, stdout, stderr)
		return
	}

	wantFile := strings.ToValidUTF8(path, "\uFFFD")
	e := entries[0]
	if e.File != wantFile {
		t.Errorf("run(%v) entry file mismatch:\ngot %q, want %q\nexit code: got %d\nstdout: got %q\nstderr: got %q", args, e.File, wantFile, code, stdout, stderr)
	}
	if e.Line != 2 {
		t.Errorf("run(%v) entry line mismatch:\ngot %d, want 2\nexit code: got %d\nstdout: got %q\nstderr: got %q", args, e.Line, code, stdout, stderr)
	}
	if e.Col != 5 {
		t.Errorf("run(%v) entry col mismatch:\ngot %d, want 5\nexit code: got %d\nstdout: got %q\nstderr: got %q", args, e.Col, code, stdout, stderr)
	}
	if e.Code != "L102" {
		t.Errorf("run(%v) entry code mismatch:\ngot %q, want %q\nexit code: got %d\nstdout: got %q\nstderr: got %q", args, e.Code, "L102", code, stdout, stderr)
	}
}
