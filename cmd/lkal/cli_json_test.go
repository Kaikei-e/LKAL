package main

import (
	"fmt"
	"testing"
)

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
