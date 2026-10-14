package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"bytes"
	"testing"
)

func TestDelete(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		fileParsedName string
		path           string
		wantFormat     string
		wantErrCode    string
		wantFile       string
		wantMsg        string
		wantHint       string
		wantErrorStr   string
	}{
		{
			name: "remove_assignment_with_comments",
			src: `@lkal "0.1"

### comment on a
a = 1
b = 2
`,
			path: "a",
			wantFormat: `@lkal "0.1"

b = 2
`,
		},
		{
			name: "blank_line_transfer_to_next_entry",
			src: `@lkal "0.1"

a = 1

### comment on b
b = 2
c = 3
`,
			path: "b",
			wantFormat: `@lkal "0.1"

a = 1

c = 3
`,
		},
		{
			name: "blank_line_transfer_to_trailing_comment",
			src: `@lkal "0.1"

a = 1

b = 2
### trailing comment
`,
			path: "b",
			wantFormat: `@lkal "0.1"

a = 1

### trailing comment
`,
		},
		{
			name: "remove_record_entry",
			src: `@lkal "0.1"

a = 1
rec {
  x = 10
}
b = 2
`,
			path: "rec",
			wantFormat: `@lkal "0.1"

a = 1
b = 2
`,
		},
		{
			name: "remove_whole_collection_key",
			src: `@lkal "0.1"

a = 1
item "one" {
  x = 1
}
item "two" {
  x = 2
}
b = 2
`,
			path: "item",
			wantFormat: `@lkal "0.1"

a = 1
b = 2
`,
		},
		{
			name: "remove_one_element",
			src: `@lkal "0.1"

item "one" {
  x = 1
}
item "two" {
  x = 2
}
`,
			path: "item/one",
			wantFormat: `@lkal "0.1"

item "two" {
  x = 2
}
`,
		},
		{
			name: "remove_last_element_collection_disappears",
			src: `@lkal "0.1"

a = 1
item "only" {
  x = 1
}
b = 2
`,
			path: "item/only",
			wantFormat: `@lkal "0.1"

a = 1
b = 2
`,
		},
		{
			name: "nested_path",
			src: `@lkal "0.1"

service "web" {
  container "app" {
    image = "alpine"
    memory = 512Mi
  }
}
`,
			path: "service/web/container/app/memory",
			wantFormat: `@lkal "0.1"

service "web" {
  container "app" {
    image = "alpine"
  }
}
`,
		},
		{
			name: "l401_error_leaves_file_unchanged",
			src: `@lkal "0.1"

a = 1
`,
			path:        "/a",
			wantErrCode: "L401",
			wantFile:    "test.lkal",
			wantMsg:     "malformed path",
			wantHint:    "separate segments with / and quote segments that contain other characters",
		},
		{
			name: "l402_error_leaves_file_unchanged",
			src: `@lkal "0.1"

a = 1
`,
			path:        "missing",
			wantErrCode: "L402",
			wantFile:    "test.lkal",
			wantMsg:     "path not found",
			wantHint:    "check the path at segment \"missing\"",
		},
		{
			name: "l402_empty_file_name_from_json_style",
			src: `@lkal "0.1"

a = 1
`,
			fileParsedName: "",
			path:           "missing",
			wantErrCode:    "L402",
			wantFile:       "",
			wantMsg:        "path not found",
			wantHint:       "check the path at segment \"missing\"",
			wantErrorStr:   "L402 path not found\n  hint: check the path at segment \"missing\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fileName := "test.lkal"
			if tc.wantErrCode != "" && tc.name == "l402_empty_file_name_from_json_style" {
				fileName = ""
			}
			f := pathTParse(t, fileName, tc.src)
			origFormat := f.Format()
			err := f.Delete(tc.path)
			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatalf("Delete(%q) succeeded unexpectedly, want error", tc.path)
				}
				errList, ok := err.(*ErrorList)
				if !ok || len(errList.Errors) != 1 {
					t.Fatalf("expected *ErrorList with 1 error, got %v", err)
				}
				e := errList.Errors[0]
				if e.Code != tc.wantErrCode {
					t.Errorf("Code = %q, want %q", e.Code, tc.wantErrCode)
				}
				if e.File != tc.wantFile {
					t.Errorf("File = %q, want %q", e.File, tc.wantFile)
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
				if !bytes.Equal(f.Format(), origFormat) {
					t.Errorf("File changed on Delete error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Delete(%q) unexpected error: %v", tc.path, err)
			}
			gotFormat := string(f.Format())
			if gotFormat != tc.wantFormat {
				t.Errorf("Format() mismatch:\ngot:\n%s\nwant:\n%s", gotFormat, tc.wantFormat)
			}
			parsed := pathTParse(t, "", tc.wantFormat)
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())
		})
	}
}
