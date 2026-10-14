package parser_test

import . "github.com/Kaikei-e/LKAL"

import (
	"strings"
	"testing"
)

type errTestCase struct {
	name              string
	src               string
	wantCode          string
	wantPos           Pos
	wantMsg           string
	wantHint          string
	checkNonEmptyHint bool
}

func errPos(line, col int) Pos {
	return Pos{Line: line, Col: col}
}

func errRequireErrorList(t *testing.T, err error) *ErrorList {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	errList, ok := err.(*ErrorList)
	if !ok {
		t.Fatalf("expected *ErrorList, got %T: %v", err, err)
	}
	return errList
}

func errRequireSingle(t *testing.T, err error) *Error {
	t.Helper()
	errList := errRequireErrorList(t, err)
	if len(errList.Errors) != 1 {
		t.Fatalf("expected exactly 1 error, got %d: %v", len(errList.Errors), errList.Error())
	}
	return errList.Errors[0]
}

func errAssertError(t *testing.T, got *Error, wantCode string, wantPos Pos, wantMsg string, wantHint string, checkNonEmptyHint bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("expected non-nil Error")
	}
	if got.Code != wantCode {
		t.Errorf("code mismatch: got %q, want %q", got.Code, wantCode)
	}
	if got.Pos != wantPos {
		t.Errorf("pos mismatch: got %v, want %v", got.Pos, wantPos)
	}
	if wantMsg != "" && got.Msg != wantMsg {
		t.Errorf("msg mismatch: got %q, want %q", got.Msg, wantMsg)
	}
	if checkNonEmptyHint {
		if strings.TrimSpace(got.Hint) == "" {
			t.Errorf("expected non-empty hint, got %q", got.Hint)
		}
	} else if wantHint != "" && got.Hint != wantHint {
		t.Errorf("hint mismatch: got %q, want %q", got.Hint, wantHint)
	}
}

func errCheckSingle(t *testing.T, err error, wantCode string, wantPos Pos, wantMsg string, wantHint string, checkNonEmptyHint bool) {
	t.Helper()
	single := errRequireSingle(t, err)
	errAssertError(t, single, wantCode, wantPos, wantMsg, wantHint, checkNonEmptyHint)
}

type expectedError struct {
	code string
	line int
	col  int
}

func assertExactErrors(t *testing.T, name, src string, want []expectedError) {
	t.Helper()
	file, err := Parse(name, []byte(src))
	if err == nil {
		t.Fatalf("Parse(%q) succeeded unexpectedly, want %d error(s)", name, len(want))
	}
	if file != nil {
		t.Errorf("Parse(%q) returned non-nil File on error: %v", name, file)
	}
	errList, ok := err.(*ErrorList)
	if !ok {
		t.Fatalf("Parse(%q) returned error of type %T, want *ErrorList", name, err)
	}
	if len(errList.Errors) != len(want) {
		t.Fatalf("Parse(%q) returned %d error(s), want %d:\ngot:\n%s", name, len(errList.Errors), len(want), errList.Error())
	}
	for i, w := range want {
		got := errList.Errors[i]
		if got.Code != w.code || got.Pos.Line != w.line || got.Pos.Col != w.col {
			t.Errorf("Parse(%q) error[%d] mismatch: got %s at %d:%d, want %s at %d:%d",
				name, i, got.Code, got.Pos.Line, got.Pos.Col, w.code, w.line, w.col)
		}
	}
}

func assertSingleErrorHint(t *testing.T, name, src, wantCode string, line, col int, wantHint string) {
	t.Helper()
	file, err := Parse(name, []byte(src))
	if err == nil {
		t.Fatalf("Parse(%q) succeeded unexpectedly, want error", name)
	}
	if file != nil {
		t.Errorf("Parse(%q) returned non-nil File on error: %v", name, file)
	}
	single := errRequireSingle(t, err)
	if single.Code != wantCode {
		t.Errorf("Parse(%q) code mismatch: got %q, want %q", name, single.Code, wantCode)
	}
	if single.Pos.Line != line || single.Pos.Col != col {
		t.Errorf("Parse(%q) pos mismatch: got %d:%d, want %d:%d", name, single.Pos.Line, single.Pos.Col, line, col)
	}
	if single.Hint != wantHint {
		t.Errorf("Parse(%q) hint mismatch:\ngot:  %q\nwant: %q", name, single.Hint, wantHint)
	}
}

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

func TestWebLkalExactExample(t *testing.T) {
	var b strings.Builder
	b.WriteString("@lkal \"0.1\"\n")
	for i := 2; i <= 11; i++ {
		b.WriteString("\n")
	}
	b.WriteString("port =    tcp\n")

	file, err := Parse("web.lkal", []byte(b.String()))
	if err == nil {
		t.Fatalf("expected error from Parse, got file: %v", file)
	}

	errList := errRequireErrorList(t, err)
	if len(errList.Errors) != 1 {
		t.Fatalf("expected exactly 1 error, got %d: %v", len(errList.Errors), errList.Error())
	}

	gotSingle := errList.Errors[0]
	errAssertError(t, gotSingle, "L102", errPos(12, 11), "bare word in value position", `write "tcp" for a string or .tcp for an enum`, false)

	wantRender := "web.lkal:12:11: L102 bare word in value position\n  hint: write \"tcp\" for a string or .tcp for an enum"
	if got := err.Error(); got != wantRender {
		t.Errorf("err.Error() mismatch:\ngot:  %q\nwant: %q", got, wantRender)
	}
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
