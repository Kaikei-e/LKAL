package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"os"
	"testing"
)

func TestGetErrors(t *testing.T) {
	data, err := os.ReadFile("../../testdata/conformance/valid/spec_example.lkal")
	if err != nil {
		t.Fatalf("failed to read spec_example.lkal: %v", err)
	}
	f := pathTParse(t, "spec_example.lkal", string(data))

	const malformedHint = "separate segments with / and quote segments that contain other characters"

	tests := []struct {
		name         string
		file         *File
		fileName     string
		path         string
		wantCode     string
		wantMsg      string
		wantHint     string
		wantErrorStr string
	}{
		{
			name:     "empty_path",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "leading_slash",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "/a",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "trailing_slash",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a/",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "double_slash",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a//b",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "space_in_segment",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a b",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "unclosed_quote",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "\"open",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "invalid_escape_in_segment",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a/\"\\q\"",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "invalid_character_in_segment",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a/%",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "missing_key_in_root",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "missing",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"missing\"",
		},
		{
			name:     "missing_key_in_record",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/web/missing",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"missing\"",
		},
		{
			name:     "missing_label_in_collection",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/missing",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"missing\"",
		},
		{
			name:     "segment_after_scalar",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/web/replicas/x",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"x\"",
		},
		{
			name:     "segment_after_list",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/web/container/app/args/x",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"x\"",
		},
		{
			name:     "l402_escaped_segment_in_hint",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/\"x\\u{41}\"",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"xA\"",
		},
		{
			name:         "exact_error_string_api",
			file:         pathTParse(t, "web.lkal", "@lkal \"0.1\"\n"),
			fileName:     "web.lkal",
			path:         "api",
			wantCode:     "L402",
			wantMsg:      "path not found",
			wantHint:     "check the path at segment \"api\"",
			wantErrorStr: "web.lkal: L402 path not found\n  hint: check the path at segment \"api\"",
		},
		{
			name:         "l402_empty_file_name_from_json_style",
			file:         pathTParse(t, "", "@lkal \"0.1\"\n"),
			fileName:     "",
			path:         "api",
			wantCode:     "L402",
			wantMsg:      "path not found",
			wantHint:     "check the path at segment \"api\"",
			wantErrorStr: "L402 path not found\n  hint: check the path at segment \"api\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := tc.file.Get(tc.path)
			if err == nil {
				t.Fatalf("Get(%q) succeeded unexpectedly, got %v", tc.path, val)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("Get(%q) error type = %T, want *ErrorList", tc.path, err)
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("Get(%q) error count = %d, want 1", tc.path, len(errList.Errors))
			}
			e := errList.Errors[0]
			if e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", e.Code, tc.wantCode)
			}
			if e.File != tc.fileName {
				t.Errorf("File = %q, want %q", e.File, tc.fileName)
			}
			if e.Pos != (Pos{}) {
				t.Errorf("Pos = %v, want zero Pos", e.Pos)
			}
			if e.Msg != tc.wantMsg {
				t.Errorf("Msg = %q, want %q", e.Msg, tc.wantMsg)
			}
			if e.Hint != tc.wantHint {
				t.Errorf("Hint = %q, want %q", e.Hint, tc.wantHint)
			}
			if tc.wantErrorStr != "" && e.Error() != tc.wantErrorStr {
				t.Errorf("Error() =\n%q\nwant:\n%q", e.Error(), tc.wantErrorStr)
			}
		})
	}
}
