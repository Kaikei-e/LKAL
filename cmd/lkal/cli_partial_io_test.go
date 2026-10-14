package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPartialIOErrors(t *testing.T) {
	const (
		badContent           = "@lkal \"0.1\"\na = b\n"
		goodContent          = "@lkal \"0.1\"\na = 1\n"
		nonCanonOrigContent  = "@lkal \"0.1\"\na = 1, b = [1,2]\n"
		nonCanonCanonContent = "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
	)

	tests := []struct {
		name           string
		buildArgs      func(bad, good, nonCanon, missing string) []string
		wantStdout     func(canonical string) string
		wantExactErr   bool
		onlyLKALLine   bool
		wantJSONErr    bool
		checkRewritten bool
	}{
		{
			name: "check bad.lkal missing.lkal",
			buildArgs: func(bad, good, nonCanon, missing string) []string {
				return []string{"check", bad, missing}
			},
			wantStdout:   func(string) string { return "" },
			wantExactErr: true,
		},
		{
			name: "check missing.lkal bad.lkal",
			buildArgs: func(bad, good, nonCanon, missing string) []string {
				return []string{"check", missing, bad}
			},
			wantStdout:   func(string) string { return "" },
			wantExactErr: true,
		},
		{
			name: "check good.lkal missing.lkal",
			buildArgs: func(bad, good, nonCanon, missing string) []string {
				return []string{"check", good, missing}
			},
			wantStdout:   func(string) string { return "" },
			onlyLKALLine: true,
		},
		{
			name: "check --json bad.lkal missing.lkal",
			buildArgs: func(bad, good, nonCanon, missing string) []string {
				return []string{"check", "--json", bad, missing}
			},
			wantStdout:  func(string) string { return "" },
			wantJSONErr: true,
		},
		{
			name: "fmt good_noncanonical.lkal missing.lkal",
			buildArgs: func(bad, good, nonCanon, missing string) []string {
				return []string{"fmt", nonCanon, missing}
			},
			wantStdout: func(canonical string) string { return canonical },
		},
		{
			name: "fmt -w good_noncanonical.lkal missing.lkal",
			buildArgs: func(bad, good, nonCanon, missing string) []string {
				return []string{"fmt", "-w", nonCanon, missing}
			},
			wantStdout:     func(string) string { return "" },
			checkRewritten: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bad := writeTemp(t, dir, "bad.lkal", badContent)
			good := writeTemp(t, dir, "good.lkal", goodContent)
			nonCanon := writeTemp(t, dir, "good_noncanonical.lkal", nonCanonOrigContent)
			missing := filepath.Join(dir, "missing.lkal")

			args := tc.buildArgs(bad, good, nonCanon, missing)
			code, stdout, stderr := execRun(t, args)

			wantCode := 2
			if code != wantCode {
				t.Errorf("run(%v) exit code mismatch:\ngot %d, want %d\nstdout: got %q\nstderr: got %q",
					args, code, wantCode, stdout, stderr)
			}

			expectedStdout := tc.wantStdout(nonCanonCanonContent)
			if stdout != expectedStdout {
				t.Errorf("run(%v) stdout mismatch:\ngot %q, want %q\nexit code: got %d\nstderr: got %q",
					args, stdout, expectedStdout, code, stderr)
			}

			var lkalLineIdx = -1
			var jsonLineIdx = -1

			if tc.onlyLKALLine {
				lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
				if len(lines) != 1 || !strings.HasPrefix(lines[0], "lkal: ") || !strings.Contains(lines[0], missing) {
					t.Errorf("run(%v) stderr want only 'lkal: ' line mentioning %q:\ngot %q\nexit code: got %d\nstdout: got %q",
						args, missing, stderr, code, stdout)
				}
			} else {
				lines := strings.Split(stderr, "\n")
				for idx, line := range lines {
					if strings.HasPrefix(line, "lkal: ") && strings.Contains(line, missing) {
						lkalLineIdx = idx
						break
					}
				}
				if lkalLineIdx == -1 {
					t.Errorf("run(%v) stderr missing line starting with 'lkal: ' mentioning %q:\ngot %q\nexit code: got %d\nstdout: got %q",
						args, missing, stderr, code, stdout)
				}
			}

			if tc.wantExactErr {
				exactBlock := bad + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
				if !strings.Contains(stderr, exactBlock) {
					t.Errorf("run(%v) stderr missing exact error block %q:\ngot %q\nexit code: got %d\nstdout: got %q",
						args, exactBlock, stderr, code, stdout)
				}
			}

			if tc.wantJSONErr {
				type errEntry struct {
					File    string `json:"file"`
					Line    int    `json:"line"`
					Col     int    `json:"col"`
					Code    string `json:"code"`
					Message string `json:"message"`
					Hint    string `json:"hint"`
				}

				var foundJSON []errEntry
				lines := strings.Split(stderr, "\n")
				for idx, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
						var parsed []errEntry
						if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
							foundJSON = parsed
							jsonLineIdx = idx
							break
						}
					}
				}
				if foundJSON == nil {
					t.Errorf("run(%v) stderr missing valid JSON array line:\ngot %q\nexit code: got %d\nstdout: got %q",
						args, stderr, code, stdout)
				} else if len(foundJSON) != 1 {
					t.Errorf("run(%v) JSON array entry count: got %d, want 1\nexit code: got %d\nstdout: got %q\nstderr: got %q",
						args, len(foundJSON), code, stdout, stderr)
				} else {
					e := foundJSON[0]
					if e.File != bad || e.Line != 2 || e.Col != 5 || e.Code != "L102" || e.Message != "bare word in value position" || e.Hint != "write \"b\" for a string or .b for an enum" {
						t.Errorf("run(%v) JSON error entry mismatch:\ngot %+v\nexit code: got %d\nstdout: got %q\nstderr: got %q",
							args, e, code, stdout, stderr)
					}
				}
				if lkalLineIdx != -1 && jsonLineIdx != -1 && lkalLineIdx == jsonLineIdx {
					t.Errorf("run(%v) JSON array and 'lkal: ' line must be separate lines\nexit code: got %d\nstdout: got %q\nstderr: got %q",
						args, code, stdout, stderr)
				}
			}

			if tc.checkRewritten {
				data, err := os.ReadFile(nonCanon)
				if err != nil {
					t.Fatalf("os.ReadFile(%q): %v\nexit code: got %d\nstdout: got %q\nstderr: got %q",
						nonCanon, err, code, stdout, stderr)
				}
				if string(data) != nonCanonCanonContent {
					t.Errorf("rewritten file %q mismatch:\ngot %q, want %q\nexit code: got %d\nstdout: got %q\nstderr: got %q",
						nonCanon, string(data), nonCanonCanonContent, code, stdout, stderr)
				}
			} else {
				data, err := os.ReadFile(nonCanon)
				if err == nil && string(data) != nonCanonOrigContent {
					t.Errorf("file %q unexpectedly modified:\ngot %q, want %q\nexit code: got %d\nstdout: got %q\nstderr: got %q",
						nonCanon, string(data), nonCanonOrigContent, code, stdout, stderr)
				}
			}
		})
	}
}
