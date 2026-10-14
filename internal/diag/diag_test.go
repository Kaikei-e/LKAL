package diag

import "testing"

func TestErrorRendering(t *testing.T) {
	t.Run("ErrorWithoutHint", func(t *testing.T) {
		err := &Error{
			File: "doc.lkal",
			Pos:  Pos{Line: 1, Col: 5},
			Code: "L205",
			Msg:  "key starts with $",
		}
		want := "doc.lkal:1:5: L205 key starts with $"
		if got := err.Error(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("ErrorWithHint", func(t *testing.T) {
		err := &Error{
			File: "doc.lkal",
			Pos:  Pos{Line: 2, Col: 3},
			Code: "L101",
			Msg:  "entry header broken across lines",
			Hint: "keep the key, label and { on one line",
		}
		want := "doc.lkal:2:3: L101 entry header broken across lines\n  hint: keep the key, label and { on one line"
		if got := err.Error(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("ErrorListEmpty", func(t *testing.T) {
		list := &ErrorList{}
		if got := list.Error(); got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("ErrorListSingle", func(t *testing.T) {
		err := &Error{
			File: "doc.lkal",
			Pos:  Pos{Line: 1, Col: 2},
			Code: "L201",
			Msg:  "duplicate key or label",
			Hint: "remove one of them",
		}
		list := &ErrorList{Errors: []*Error{err}}
		want := err.Error()
		if got := list.Error(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("ErrorListMultiple", func(t *testing.T) {
		err1 := &Error{
			File: "doc.lkal",
			Pos:  Pos{Line: 1, Col: 2},
			Code: "L201",
			Msg:  "first msg",
		}
		err2 := &Error{
			File: "doc.lkal",
			Pos:  Pos{Line: 2, Col: 4},
			Code: "L202",
			Msg:  "second msg",
			Hint: "second hint",
		}
		list := &ErrorList{Errors: []*Error{err1, err2}}
		want := err1.Error() + "\n" + err2.Error()
		if got := list.Error(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("DirectWebLkalExample", func(t *testing.T) {
		err := &Error{
			File: "web.lkal",
			Pos:  Pos{Line: 12, Col: 11},
			Code: "L102",
			Msg:  "bare word in value position",
			Hint: `write "tcp" for a string or .tcp for an enum`,
		}
		want := "web.lkal:12:11: L102 bare word in value position\n  hint: write \"tcp\" for a string or .tcp for an enum"
		if got := err.Error(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestErrorRenderingWithoutFileOrPos(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "file_and_pos_with_hint",
			err: &Error{
				File: "web.lkal",
				Pos:  Pos{Line: 12, Col: 11},
				Code: "L102",
				Msg:  "bare word in value position",
				Hint: `write "tcp" for a string or .tcp for an enum`,
			},
			want: "web.lkal:12:11: L102 bare word in value position\n  hint: write \"tcp\" for a string or .tcp for an enum",
		},
		{
			name: "file_and_pos_without_hint",
			err: &Error{
				File: "web.lkal",
				Pos:  Pos{Line: 12, Col: 11},
				Code: "L102",
				Msg:  "bare word in value position",
			},
			want: "web.lkal:12:11: L102 bare word in value position",
		},
		{
			name: "pos_only_with_hint",
			err: &Error{
				File: "",
				Pos:  Pos{Line: 3, Col: 5},
				Code: "L302",
				Msg:  "null value",
				Hint: "omit the key instead of writing null",
			},
			want: "3:5: L302 null value\n  hint: omit the key instead of writing null",
		},
		{
			name: "pos_only_without_hint",
			err: &Error{
				File: "",
				Pos:  Pos{Line: 3, Col: 5},
				Code: "L302",
				Msg:  "null value",
			},
			want: "3:5: L302 null value",
		},
		{
			name: "file_only_with_hint",
			err: &Error{
				File: "web.lkal",
				Pos:  Pos{},
				Code: "L402",
				Msg:  "path not found",
				Hint: `check the path at segment "api"`,
			},
			want: "web.lkal: L402 path not found\n  hint: check the path at segment \"api\"",
		},
		{
			name: "file_only_without_hint",
			err: &Error{
				File: "web.lkal",
				Pos:  Pos{},
				Code: "L402",
				Msg:  "path not found",
			},
			want: "web.lkal: L402 path not found",
		},
		{
			name: "neither_with_hint",
			err: &Error{
				File: "",
				Pos:  Pos{},
				Code: "L402",
				Msg:  "path not found",
				Hint: `check the path at segment "api"`,
			},
			want: "L402 path not found\n  hint: check the path at segment \"api\"",
		},
		{
			name: "neither_without_hint",
			err: &Error{
				File: "",
				Pos:  Pos{},
				Code: "L402",
				Msg:  "path not found",
			},
			want: "L402 path not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() mismatch:\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestErrorCatalogJSONAndPath(t *testing.T) {
	type errCatCase struct {
		code      string
		wantMsg   string
		wantHint  string
		checkHint bool
	}

	tests := []errCatCase{
		{code: "L301", wantMsg: "invalid JSON", wantHint: "fix the JSON syntax", checkHint: true},
		{code: "L302", wantMsg: "null value", wantHint: "omit the key instead of writing null", checkHint: true},
		{code: "L303", wantMsg: "number in exponent notation", wantHint: "write the number without an exponent", checkHint: true},
		{code: "L304", wantMsg: "list element is not a scalar", wantHint: "use only scalars as list elements", checkHint: true},
		{code: "L305", wantMsg: "malformed tag", checkHint: false},
		{code: "L401", wantMsg: "malformed path", wantHint: "separate segments with / and quote segments that contain other characters", checkHint: true},
		{code: "L402", wantMsg: "path not found", checkHint: false},
		{code: "L403", wantMsg: "invalid value", checkHint: false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := NewError("", Pos{}, tt.code, "")
			if err == nil {
				t.Fatalf("NewError(%q) returned nil", tt.code)
			}
			if err.Code != tt.code {
				t.Errorf("code mismatch: got %q, want %q", err.Code, tt.code)
			}
			if err.Msg != tt.wantMsg {
				t.Errorf("msg mismatch for %s: got %q, want %q", tt.code, err.Msg, tt.wantMsg)
			}
			if tt.checkHint && err.Hint != tt.wantHint {
				t.Errorf("hint mismatch for %s: got %q, want %q", tt.code, err.Hint, tt.wantHint)
			}
		})
	}
}

func TestErrorCatalogL107(t *testing.T) {
	err := NewError("", Pos{}, "L107", "")
	if err == nil {
		t.Fatalf("NewError(\"L107\") returned nil")
	}
	if err.Code != "L107" {
		t.Errorf("code mismatch: got %q, want %q", err.Code, "L107")
	}
	if err.Msg != "blocks nested too deeply" {
		t.Errorf("msg mismatch: got %q, want %q", err.Msg, "blocks nested too deeply")
	}
	if err.Hint != "nest blocks at most 1000 levels deep" {
		t.Errorf("hint mismatch: got %q, want %q", err.Hint, "nest blocks at most 1000 levels deep")
	}
}
