package lkal

import (
	"strconv"
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
			Code: "L204",
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

func TestSyntaxErrors(t *testing.T) {
	tests := []errTestCase{
		{
			name:     "L101_NewlineAfterAssignKey",
			src:      "@lkal \"0.1\"\nkey\n  = 1\n",
			wantCode: "L101",
			wantPos:  errPos(2, 4),
			wantMsg:  "entry header broken across lines",
			wantHint: "keep the key, label and { on one line",
		},
		{
			name:     "L101_NewlineAfterRecordKey",
			src:      "@lkal \"0.1\"\nserver\n{\n}\n",
			wantCode: "L101",
			wantPos:  errPos(2, 7),
			wantMsg:  "entry header broken across lines",
			wantHint: "keep the key, label and { on one line",
		},
		{
			name:     "L101_NewlineAfterLabel",
			src:      "@lkal \"0.1\"\nport \"http\"\n{\n}\n",
			wantCode: "L101",
			wantPos:  errPos(2, 12),
			wantMsg:  "entry header broken across lines",
			wantHint: "keep the key, label and { on one line",
		},
		{
			name:     "L101_NewlineAfterUnicodeKey",
			src:      "@lkal \"0.1\"\n\"設定\"\n  = 1\n",
			wantCode: "L101",
			wantPos:  errPos(2, 5),
			wantMsg:  "entry header broken across lines",
			wantHint: "keep the key, label and { on one line",
		},
		{
			name:     "L102_BareWordAssign",
			src:      "@lkal \"0.1\"\nproto = tcp\n",
			wantCode: "L102",
			wantPos:  errPos(2, 9),
			wantMsg:  "bare word in value position",
			wantHint: `write "tcp" for a string or .tcp for an enum`,
		},
		{
			name:     "L102_BareWordInList",
			src:      "@lkal \"0.1\"\narr = [foo]\n",
			wantCode: "L102",
			wantPos:  errPos(2, 8),
			wantMsg:  "bare word in value position",
			wantHint: `write "foo" for a string or .foo for an enum`,
		},
		{
			name:     "L102_BareWordUnicodeKey",
			src:      "@lkal \"0.1\"\n\"設定\" = bar\n",
			wantCode: "L102",
			wantPos:  errPos(2, 8),
			wantMsg:  "bare word in value position",
			wantHint: `write "bar" for a string or .bar for an enum`,
		},
		{
			name:     "L103_UnclosedRecord",
			src:      "@lkal \"0.1\"\nrec {\n  x = 1\n",
			wantCode: "L103",
			wantPos:  errPos(2, 5),
			wantMsg:  "unclosed bracket",
			wantHint: "add the matching closing bracket",
		},
		{
			name:     "L103_UnclosedList",
			src:      "@lkal \"0.1\"\narr = [1, 2\n",
			wantCode: "L103",
			wantPos:  errPos(2, 7),
			wantMsg:  "unclosed bracket",
			wantHint: "add the matching closing bracket",
		},
		{
			name:     "L104_BlankLineInRawString",
			src:      "@lkal \"0.1\"\ns =\n  \\\\line1\n\n  \\\\line2\n",
			wantCode: "L104",
			wantPos:  errPos(5, 3),
			wantMsg:  "blank line or comment inside a multi-line string",
			wantHint: `remove the gap; write an empty line as a lone \\`,
		},
		{
			name:     "L104_CommentLineInRawString",
			src:      "@lkal \"0.1\"\ns =\n  \\\\line1\n  ### gap comment\n  \\\\line2\n",
			wantCode: "L104",
			wantPos:  errPos(5, 3),
			wantMsg:  "blank line or comment inside a multi-line string",
			wantHint: `remove the gap; write an empty line as a lone \\`,
		},
		{
			name:     "L104_NoIndentGap",
			src:      "@lkal \"0.1\"\ns =\n\\\\line1\n\n\\\\line2\n",
			wantCode: "L104",
			wantPos:  errPos(5, 1),
			wantMsg:  "blank line or comment inside a multi-line string",
			wantHint: `remove the gap; write an empty line as a lone \\`,
		},
		{
			name:     "L105_MissingVersionNonEmpty",
			src:      "key = 1\n",
			wantCode: "L105",
			wantPos:  errPos(1, 1),
			wantMsg:  "missing, misplaced or repeated version declaration",
			wantHint: `write @lkal "0.1" once, at the top`,
		},
		{
			name:     "L105_MissingVersionEmpty",
			src:      "",
			wantCode: "L105",
			wantPos:  errPos(1, 1),
			wantMsg:  "missing, misplaced or repeated version declaration",
			wantHint: `write @lkal "0.1" once, at the top`,
		},
		{
			name:     "L105_MissingVersionOnlyComments",
			src:      "### just a comment\n",
			wantCode: "L105",
			wantPos:  errPos(2, 1),
			wantMsg:  "missing, misplaced or repeated version declaration",
			wantHint: `write @lkal "0.1" once, at the top`,
		},
		{
			name:     "L105_DuplicateVersion",
			src:      "@lkal \"0.1\"\n@lkal \"0.1\"\n",
			wantCode: "L105",
			wantPos:  errPos(2, 1),
			wantMsg:  "missing, misplaced or repeated version declaration",
			wantHint: `write @lkal "0.1" once, at the top`,
		},
		{
			name:     "L105_NonStringVersionInt",
			src:      "@lkal 1\n",
			wantCode: "L105",
			wantPos:  errPos(1, 7),
			wantMsg:  "missing, misplaced or repeated version declaration",
			wantHint: `write @lkal "0.1" once, at the top`,
		},
		{
			name:     "L105_NonStringVersionBool",
			src:      "@lkal true\n",
			wantCode: "L105",
			wantPos:  errPos(1, 7),
			wantMsg:  "missing, misplaced or repeated version declaration",
			wantHint: `write @lkal "0.1" once, at the top`,
		},
		{
			name:              "L106_ReservedWordTrueKey",
			src:               "@lkal \"0.1\"\ntrue = 1\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 1),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
		{
			name:              "L106_ReservedWordFalseKey",
			src:               "@lkal \"0.1\"\nfalse = 2\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 1),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
		{
			name:              "L106_BraceAfterAssign",
			src:               "@lkal \"0.1\"\nk = {\n}\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 5),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
		{
			name:              "L106_CommentBetweenAssignAndValue",
			src:               "@lkal \"0.1\"\nk =\n### comment\n",
			wantCode:          "L106",
			wantPos:           errPos(3, 1),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
		{
			name:              "L106_MissingCommaInList",
			src:               "@lkal \"0.1\"\nlist = [1 2]\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 11),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
		{
			name:              "L106_NestedList",
			src:               "@lkal \"0.1\"\nlist = [[]]\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 9),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
		{
			name:              "L106_ExtraTokenAfterValue",
			src:               "@lkal \"0.1\"\na = 1 b = 2\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 7),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
		{
			name:              "L106_RawLineWithoutPrecedingRaw",
			src:               "@lkal \"0.1\"\n\\\\orphan\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 1),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse(tt.name, []byte(tt.src))
			if err == nil {
				t.Fatalf("expected error, got file: %v", file)
			}
			errCheckSingle(t, err, tt.wantCode, tt.wantPos, tt.wantMsg, tt.wantHint, tt.checkNonEmptyHint)
		})
	}
}

func TestSemanticErrors(t *testing.T) {
	tests := []errTestCase{
		{
			name:     "L201_DuplicateAssignKey",
			src:      "@lkal \"0.1\"\na = 1\na = 2\n",
			wantCode: "L201",
			wantPos:  errPos(3, 1),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_DuplicateRecordKey",
			src:      "@lkal \"0.1\"\nrec {}\nrec {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 1),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_DuplicateCollectionLabel",
			src:      "@lkal \"0.1\"\ncoll \"a\" {}\ncoll \"a\" {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 6),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_DuplicateUnicodeLabel",
			src:      "@lkal \"0.1\"\nsvc \"日本語\" {}\nsvc \"日本語\" {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 5),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_AssignThenRecord",
			src:      "@lkal \"0.1\"\nk = 1\nk {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 1),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_RecordThenLabeledBlock",
			src:      "@lkal \"0.1\"\nk {}\nk \"a\" {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 1),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_LabeledBlockThenAssign",
			src:      "@lkal \"0.1\"\nk \"a\" {}\nk = 1\n",
			wantCode: "L201",
			wantPos:  errPos(3, 1),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_MultiLabelDuplicate",
			src:      "@lkal \"0.1\"\nroute \"GET\" \"/users\" {}\nroute \"GET\" \"/users\" {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 13),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L204_IntAndString",
			src:      "@lkal \"0.1\"\nl = [1, \"two\"]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 9),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L204_IntAndDecimal",
			src:      "@lkal \"0.1\"\nl = [1, 2.5]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 9),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L204_DecimalAndInt",
			src:      "@lkal \"0.1\"\nl = [1.5, 2]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 11),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L204_BoolAndEnum",
			src:      "@lkal \"0.1\"\nl = [true, .active]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 12),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L204_StringAndRef",
			src:      "@lkal \"0.1\"\nl = [\"a\", &ref/one]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 11),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L205_BareDollarKey",
			src:      "@lkal \"0.1\"\n\"$foo\" = 1\n",
			wantCode: "L205",
			wantPos:  errPos(2, 1),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L205_QuotedDollarKey",
			src:      "@lkal \"0.1\"\n\"$schema\" = \"test\"\n",
			wantCode: "L205",
			wantPos:  errPos(2, 1),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L205_DollarRecordKey",
			src:      "@lkal \"0.1\"\n\"$rec\" {}\n",
			wantCode: "L205",
			wantPos:  errPos(2, 1),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L205_DollarLabeledKey",
			src:      "@lkal \"0.1\"\n\"$coll\" \"a\" {}\n",
			wantCode: "L205",
			wantPos:  errPos(2, 1),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L205_DollarLabel",
			src:      "@lkal \"0.1\"\nsvc \"$schema\" {}\n",
			wantCode: "L205",
			wantPos:  errPos(2, 5),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L207_Version02",
			src:      "@lkal \"0.2\"\n",
			wantCode: "L207",
			wantPos:  errPos(1, 7),
			wantMsg:  "unsupported version",
			wantHint: `this implementation accepts only "0.1"`,
		},
		{
			name:     "L207_Version10",
			src:      "@lkal \"1.0\"\n",
			wantCode: "L207",
			wantPos:  errPos(1, 7),
			wantMsg:  "unsupported version",
			wantHint: `this implementation accepts only "0.1"`,
		},
		{
			name:     "L207_Version010",
			src:      "@lkal \"0.10\"\n",
			wantCode: "L207",
			wantPos:  errPos(1, 7),
			wantMsg:  "unsupported version",
			wantHint: `this implementation accepts only "0.1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse(tt.name, []byte(tt.src))
			if err == nil {
				t.Fatalf("expected error, got file: %v", file)
			}
			errCheckSingle(t, err, tt.wantCode, tt.wantPos, tt.wantMsg, tt.wantHint, tt.checkNonEmptyHint)
		})
	}

	t.Run("ValidQuantityDifferentUnitsInList", func(t *testing.T) {
		file, err := Parse("units.lkal", []byte("@lkal \"0.1\"\nl = [10m, 20s]\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file == nil {
			t.Fatalf("expected non-nil File")
		}
	})

	t.Run("ValidEmptyList", func(t *testing.T) {
		file, err := Parse("empty_list.lkal", []byte("@lkal \"0.1\"\nl = []\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file == nil {
			t.Fatalf("expected non-nil File")
		}
	})

	t.Run("ValidNonContiguousAndLabeledAssign", func(t *testing.T) {
		src := "@lkal \"0.1\"\nport \"http\" {}\nhost = \"localhost\"\nport \"grpc\" {}\nitem \"lbl\" = 1\nroute \"GET\" \"/users\" { handler = \"list\" }\n"
		file, err := Parse("labeled_sugar.lkal", []byte(src))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file == nil {
			t.Fatalf("expected non-nil File")
		}
	})
}

func TestSemanticPrecedence(t *testing.T) {
	t.Run("L205_Over_L201_Conflict", func(t *testing.T) {
		src := "@lkal \"0.1\"\n\"$k\" = 1\n\"$k\" {}\n"
		file, err := Parse("prec205_201_conflict.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 2 {
			t.Fatalf("expected exactly 2 errors, got %d: %v", len(errList.Errors), errList.Error())
		}
		errAssertError(t, errList.Errors[0], "L205", errPos(2, 1), "key starts with $", "rename the key", false)
		errAssertError(t, errList.Errors[1], "L205", errPos(3, 1), "key starts with $", "rename the key", false)
	})

	t.Run("L205_Over_L201", func(t *testing.T) {
		src := "@lkal \"0.1\"\n\"$dup\" = 1\n\"$dup\" = 2\n"
		file, err := Parse("prec205_201.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 2 {
			t.Fatalf("expected exactly 2 errors, got %d: %v", len(errList.Errors), errList.Error())
		}
		errAssertError(t, errList.Errors[0], "L205", errPos(2, 1), "key starts with $", "rename the key", false)
		errAssertError(t, errList.Errors[1], "L205", errPos(3, 1), "key starts with $", "rename the key", false)
	})

	t.Run("L201_AssignAndBlock", func(t *testing.T) {
		src := "@lkal \"0.1\"\nitem = 1\nitem {}\n"
		file, err := Parse("prec201_assign_block.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errCheckSingle(t, err, "L201", errPos(3, 1), "duplicate key or label", "remove one of them", false)
	})

	t.Run("L201_NonContiguousDuplicateLabel", func(t *testing.T) {
		src := "@lkal \"0.1\"\nport \"http\" {}\nhost = \"localhost\"\nport \"http\" {}\n"
		file, err := Parse("prec201_noncontig.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errCheckSingle(t, err, "L201", errPos(4, 6), "duplicate key or label", "remove one of them", false)
	})
}

func TestInnermostL103(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantPos Pos
	}{
		{
			name:    "NestedRecordTwoLevels",
			src:     "@lkal \"0.1\"\nouter {\n  inner {\n    v = 1\n",
			wantPos: errPos(3, 9),
		},
		{
			name:    "NestedRecordThreeLevels",
			src:     "@lkal \"0.1\"\na {\n  b {\n    c {\n      v = 1\n",
			wantPos: errPos(4, 7),
		},
		{
			name:    "RecordContainingList",
			src:     "@lkal \"0.1\"\nouter {\n  list = [\n    1,\n",
			wantPos: errPos(3, 10),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse(tt.name, []byte(tt.src))
			if err == nil {
				t.Fatalf("expected error, got file: %v", file)
			}
			errCheckSingle(t, err, "L103", tt.wantPos, "unclosed bracket", "add the matching closing bracket", false)
		})
	}
}

func TestSyntaxRecovery(t *testing.T) {
	t.Run("SingleTopLevelEntryMultipleSyntaxErrors", func(t *testing.T) {
		src := "@lkal \"0.1\"\nservice {\n  a = bad1\n  b = bad2\n}\nvalid = \"ok\"\n"
		file, err := Parse("recovery_single.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errCheckSingle(t, err, "L102", errPos(3, 7), "bare word in value position", `write "bad1" for a string or .bad1 for an enum`, false)
	})

	t.Run("MultipleTopLevelEntriesRecover", func(t *testing.T) {
		src := "@lkal \"0.1\"\na = bad1\nb = bad2\nc = bad3\n"
		file, err := Parse("recovery_multi.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 3 {
			t.Fatalf("expected 3 errors, got %d", len(errList.Errors))
		}
		errAssertError(t, errList.Errors[0], "L102", errPos(2, 5), "bare word in value position", `write "bad1" for a string or .bad1 for an enum`, false)
		errAssertError(t, errList.Errors[1], "L102", errPos(3, 5), "bare word in value position", `write "bad2" for a string or .bad2 for an enum`, false)
		errAssertError(t, errList.Errors[2], "L102", errPos(4, 5), "bare word in value position", `write "bad3" for a string or .bad3 for an enum`, false)
	})

	t.Run("NestedBracesTrackDepth", func(t *testing.T) {
		src := "@lkal \"0.1\"\ntop1 {\n  nested {\n    x = bad1\n    y = bad2\n  }\n  z = bad3\n}\ntop2 = 1\n"
		file, err := Parse("recovery_nested.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errCheckSingle(t, err, "L102", errPos(4, 9), "bare word in value position", `write "bad1" for a string or .bad1 for an enum`, false)
	})
}

func TestMultilineAssignBlockRecoveryReportsSingleError(t *testing.T) {
	src := `@lkal "0.1"
a = {
  b = 1
}
`
	assertExactErrors(t, "multiline_assign_block.lkal", src, []expectedError{
		{code: "L106", line: 2, col: 5},
	})
}

func TestInterruptedMultilineRawStringReportsSingleError(t *testing.T) {
	src := `@lkal "0.1"
a =
  \\one

  \\two
  \\three
`
	assertExactErrors(t, "interrupted_multiline_raw.lkal", src, []expectedError{
		{code: "L104", line: 5, col: 3},
	})
}

func TestNestedBlockRecoveryResumesTopLevel(t *testing.T) {
	src := `@lkal "0.1"
a {
  b = 1 2
}
c = x
`
	assertExactErrors(t, "nested_recovery_resume.lkal", src, []expectedError{
		{code: "L106", line: 3, col: 9},
		{code: "L102", line: 5, col: 5},
	})
}

func TestSemanticErrorsPreservedBeforeUnclosedBlock(t *testing.T) {
	src := `@lkal "0.1"
x = 1
x = 2
a {
`
	assertExactErrors(t, "unclosed_block_semantic_preserved.lkal", src, []expectedError{
		{code: "L201", line: 3, col: 1},
		{code: "L103", line: 4, col: 3},
	})
}

func TestListRecoveryPreservesSubsequentEntry(t *testing.T) {
	src := `@lkal "0.1"
a = [1 2]
b = x
`
	assertExactErrors(t, "list_recovery_preserves_subsequent.lkal", src, []expectedError{
		{code: "L106", line: 2, col: 8},
		{code: "L102", line: 3, col: 5},
	})
}

func TestKindMixingRepeatedReportsConflictingKind(t *testing.T) {
	t.Run("element_assign_element", func(t *testing.T) {
		src := `@lkal "0.1"
a "x" {}
a = 1
a "y" {}
`
		assertExactErrors(t, "kind_mixing_elem_assign_elem.lkal", src, []expectedError{
			{code: "L201", line: 3, col: 1},
			{code: "L201", line: 4, col: 1},
		})
	})

	t.Run("record_element_record", func(t *testing.T) {
		src := `@lkal "0.1"
a {}
a "x" {}
a {}
`
		assertExactErrors(t, "kind_mixing_record_elem_record.lkal", src, []expectedError{
			{code: "L201", line: 3, col: 1},
			{code: "L201", line: 4, col: 1},
		})
	})
}

func TestLexicalSuppression(t *testing.T) {
	tests := []errTestCase{
		{
			name:     "L001_HashComment",
			src:      "@lkal \"0.1\"\n# bad comment\na = 1\n",
			wantCode: "L001",
			wantPos:  errPos(2, 1),
			wantMsg:  "comment starts with # or ##",
			wantHint: "start the comment with ###",
		},
		{
			name:     "L002_TrailingComment",
			src:      "@lkal \"0.1\"\na = 1 # bad comment\n",
			wantCode: "L002",
			wantPos:  errPos(2, 7),
			wantMsg:  "comment at the end of a line or inside a list",
			wantHint: "move the comment to its own line outside the list",
		},
		{
			name:     "L002_ListCommentLineContinuation",
			src:      "@lkal \"0.1\"\na = [\n  ### c\n  1,\n]\n",
			wantCode: "L002",
			wantPos:  errPos(3, 3),
			wantMsg:  "comment at the end of a line or inside a list",
			wantHint: "move the comment to its own line outside the list",
		},
		{
			name:     "L002_ListInlineCommentContinuation",
			src:      "@lkal \"0.1\"\na = [1, # bad\n 2]\n",
			wantCode: "L002",
			wantPos:  errPos(2, 9),
			wantMsg:  "comment at the end of a line or inside a list",
			wantHint: "move the comment to its own line outside the list",
		},
		{
			name:     "L003_BadEscape",
			src:      "@lkal \"0.1\"\na = \"bad \\x escape\"\n",
			wantCode: "L003",
			wantPos:  errPos(2, 10),
			wantMsg:  "invalid escape sequence",
			wantHint: `use \u{...}`,
		},
		{
			name:     "L004_QuantityBadUnit",
			src:      "@lkal \"0.1\"\na = 1h30m\n",
			wantCode: "L004",
			wantPos:  errPos(2, 5),
			wantMsg:  "digits after a unit",
			wantHint: "use a single unit (for example 90m)",
		},
		{
			name:     "L005_NumberTooLong",
			src:      "@lkal \"0.1\"\na = 99999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999\n",
			wantCode: "L005",
			wantPos:  errPos(2, 5),
			wantMsg:  "number out of range",
			wantHint: "keep integers within int64 and numbers within 100 digits",
		},
		{
			name:     "L006_UnknownDirective",
			src:      "@unknown \"0.1\"\na = 1\n",
			wantCode: "L006",
			wantPos:  errPos(1, 1),
			wantMsg:  "unknown directive",
			wantHint: "v0.1 supports only @lkal",
		},
		{
			name:     "L007_UnclosedString",
			src:      "@lkal \"0.1\"\na = \"unclosed\n",
			wantCode: "L007",
			wantPos:  errPos(2, 5),
			wantMsg:  "unterminated string",
			wantHint: `close the string on the same line; write multi-line strings as \\ lines`,
		},
		{
			name:     "L008_IllegalChar",
			src:      "@lkal \"0.1\"\na = %\n",
			wantCode: "L008",
			wantPos:  errPos(2, 5),
			wantMsg:  "invalid character",
			wantHint: `remove the character; write control characters as \u{...}`,
		},
		{
			name:     "L009_MalformedToken",
			src:      "@lkal \"0.1\"\na = .123\n",
			wantCode: "L009",
			wantPos:  errPos(2, 5),
			wantMsg:  "malformed token",
			wantHint: "follow the syntax for numbers, enums, references and directives",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse(tt.name, []byte(tt.src))
			if err == nil {
				t.Fatalf("expected error, got file: %v", file)
			}
			errCheckSingle(t, err, tt.wantCode, tt.wantPos, tt.wantMsg, tt.wantHint, tt.checkNonEmptyHint)
		})
	}
}

func TestInitialBOMProducesOnlyLexicalError(t *testing.T) {
	src := "\xEF\xBB\xBF@lkal \"0.1\"\na = 1\n"
	assertExactErrors(t, "initial_bom.lkal", src, []expectedError{
		{code: "L008", line: 1, col: 1},
	})
}

func TestIllegalTokenSuppressesParserErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "version_value_illegal_character",
			src:  "@lkal ;\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 7},
			},
		},
		{
			name: "version_value_unclosed_string",
			src:  "@lkal \"0.1\n",
			want: []expectedError{
				{code: "L007", line: 1, col: 7},
			},
		},
		{
			name: "version_trailer",
			src:  "@lkal \"0.1\" ;\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 13},
			},
		},
		{
			name: "key_trailer",
			src:  "@lkal \"0.1\"\na ;\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 3},
			},
		},
		{
			name: "label_trailer",
			src:  "@lkal \"0.1\"\na \"lbl\" ;\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 9},
			},
		},
		{
			name: "value_separator",
			src:  "@lkal \"0.1\"\na = 1 ;\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 7},
			},
		},
		{
			name: "block_separator",
			src:  "@lkal \"0.1\"\na {} ;\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 6},
			},
		},
		{
			name: "list_separator",
			src:  "@lkal \"0.1\"\na = [1 ;]\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 8},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestErrorLimitAndOrdering(t *testing.T) {
	t.Run("TruncateTo20Errors", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("@lkal \"0.1\"\n")
		for i := 1; i <= 25; i++ {
			b.WriteString("entry")
			b.WriteString(strconv.Itoa(i))
			b.WriteString(" = bad\n")
		}

		file, err := Parse("limit.lkal", []byte(b.String()))
		if err == nil {
			t.Fatalf("expected error from Parse, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 20 {
			t.Fatalf("expected 20 errors, got %d", len(errList.Errors))
		}
		if got := errList.Errors[0].Pos.Line; got != 2 {
			t.Errorf("first error line mismatch: got %d, want 2", got)
		}
		if got := errList.Errors[19].Pos.Line; got != 21 {
			t.Errorf("20th error line mismatch: got %d, want 21", got)
		}
	})

	t.Run("OrderingAcrossLines", func(t *testing.T) {
		src := "@lkal \"0.1\"\ne1 = bad1\ne2 = bad2\ne3 = bad3\n"
		file, err := Parse("order_lines.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 3 {
			t.Fatalf("expected 3 errors, got %d", len(errList.Errors))
		}
		for i := 0; i < len(errList.Errors)-1; i++ {
			curr, next := errList.Errors[i], errList.Errors[i+1]
			if curr.Pos.Line > next.Pos.Line {
				t.Errorf("line out of order: [%d]=%d > [%d]=%d", i, curr.Pos.Line, i+1, next.Pos.Line)
			}
		}
	})

	t.Run("OrderingAcrossColumnsOnSameLine", func(t *testing.T) {
		src := "@lkal \"0.1\"\n\"$a\" = 1, \"$b\" = 2\n"
		file, err := Parse("order_cols.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(errList.Errors))
		}
		if errList.Errors[0].Pos.Col >= errList.Errors[1].Pos.Col {
			t.Errorf("columns out of order: first col %d >= second col %d", errList.Errors[0].Pos.Col, errList.Errors[1].Pos.Col)
		}
	})
}

func TestUnicodeColumns(t *testing.T) {
	tests := []errTestCase{
		{
			name:     "CJKKey",
			src:      "@lkal \"0.1\"\n\"日本語キー\" = bad\n",
			wantCode: "L102",
			wantPos:  errPos(2, 11),
			wantMsg:  "bare word in value position",
			wantHint: `write "bad" for a string or .bad for an enum`,
		},
		{
			name:     "EmojiInStringKey",
			src:      "@lkal \"0.1\"\n\"🚀🎉\" = bad\n",
			wantCode: "L102",
			wantPos:  errPos(2, 8),
			wantMsg:  "bare word in value position",
			wantHint: `write "bad" for a string or .bad for an enum`,
		},
		{
			name:     "TabCountedAsOne",
			src:      "@lkal \"0.1\"\nk = \t\tbare\n",
			wantCode: "L102",
			wantPos:  errPos(2, 7),
			wantMsg:  "bare word in value position",
			wantHint: `write "bare" for a string or .bare for an enum`,
		},
		{
			name:              "TabIndentation",
			src:               "@lkal \"0.1\"\n\t\t= 1\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 3),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse(tt.name, []byte(tt.src))
			if err == nil {
				t.Fatalf("expected error, got file: %v", file)
			}
			errCheckSingle(t, err, tt.wantCode, tt.wantPos, tt.wantMsg, tt.wantHint, tt.checkNonEmptyHint)
		})
	}

	t.Run("InvalidUTF8BytesCountedAsOneCol", func(t *testing.T) {
		src := "@lkal \"0.1\"\n\xff\xfe%\n"
		file, err := Parse("invalid_utf8.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 3 {
			t.Fatalf("expected exactly 3 errors, got %d: %v", len(errList.Errors), errList.Error())
		}
		wantPositions := []Pos{errPos(2, 1), errPos(2, 2), errPos(2, 3)}
		for i, wantP := range wantPositions {
			errAssertError(t, errList.Errors[i], "L008", wantP, "invalid character", `remove the character; write control characters as \u{...}`, false)
		}
	})
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

func TestRecoveryRules(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "recovery_counts_brackets_and_braces",
			src:  "@lkal \"0.1\"\na = [\n  1,\n  x,\n  3,\n]\nb = 2\n",
			want: []expectedError{
				{code: "L102", line: 4, col: 3},
			},
		},
		{
			name: "lone_closing_brace_without_declaration",
			src:  "}\n",
			want: []expectedError{
				{code: "L105", line: 1, col: 1},
				{code: "L106", line: 1, col: 1},
			},
		},
		{
			name: "one_syntax_error_unclosed_list_with_comment",
			src:  "@lkal \"0.1\"\na = [\n### c\n",
			want: []expectedError{
				{code: "L002", line: 3, col: 1},
			},
		},
		{
			name: "one_syntax_error_missing_comma_after_comment",
			src:  "@lkal \"0.1\"\na = [\n### c\n1 2]\n",
			want: []expectedError{
				{code: "L002", line: 3, col: 1},
			},
		},
		{
			name: "one_syntax_error_multiple_comments_in_list",
			src:  "@lkal \"0.1\"\na = [1,\n### c\n### d\n2]\n",
			want: []expectedError{
				{code: "L002", line: 3, col: 1},
			},
		},
		{
			name: "one_syntax_error_nested_record_list_comment",
			src:  "@lkal \"0.1\"\na {\n b = [\n### c\n1]\n c = d\n}\n",
			want: []expectedError{
				{code: "L002", line: 4, col: 1},
			},
		},
		{
			name: "semantic_rules_suppressed_on_syntax_error",
			src:  "@lkal \"0.1\"\na = [1, \"x\" 2]\n",
			want: []expectedError{
				{code: "L106", line: 2, col: 13},
			},
		},
		{
			name: "scanning_order_first_token_problem",
			src:  "@lkal \"0.1\"\na = &/\"\\q\"\n",
			want: []expectedError{
				{code: "L009", line: 2, col: 5},
			},
		},
		{
			name: "entry_with_l002_skips_list_element_type_check",
			src:  "@lkal \"0.1\"\na = [\n### c\n1, \"x\"]\n",
			want: []expectedError{
				{code: "L002", line: 3, col: 1},
			},
		},
		{
			name: "entry_with_l002_not_added_to_ast_skips_duplicate_key_check",
			src:  "@lkal \"0.1\"\na = 1\na = [\n### c\n1]\n",
			want: []expectedError{
				{code: "L002", line: 4, col: 1},
			},
		},
		{
			name: "recovery_ignores_mismatched_bracket_in_block",
			src:  "@lkal \"0.1\"\na {\n]\nb = 1\n}\n",
			want: []expectedError{
				{code: "L106", line: 3, col: 1},
			},
		},
		{
			name: "recovery_ignores_mismatched_brace_in_list",
			src:  "@lkal \"0.1\"\na = [\n}\n1]\n",
			want: []expectedError{
				{code: "L106", line: 3, col: 1},
			},
		},
		{
			name: "recovery_ignores_mismatched_bracket_after_value_in_block",
			src:  "@lkal \"0.1\"\na {\n b = 1 ]\n c = 2\n}\n",
			want: []expectedError{
				{code: "L106", line: 3, col: 8},
			},
		},
		{
			name: "syntax_error_after_value_suppresses_semantic_check",
			src:  "@lkal \"0.1\"\na = [1, \"x\"] }\n",
			want: []expectedError{
				{code: "L106", line: 2, col: 14},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestDirectiveOutOfPlace(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "directive_in_value_position",
			src:  "@lkal \"0.1\"\na =\n@lkal \"0.1\"\n",
			want: []expectedError{
				{code: "L105", line: 3, col: 1},
			},
		},
		{
			name: "directive_in_list_position",
			src:  "@lkal \"0.1\"\na = [\n@lkal\n]\n",
			want: []expectedError{
				{code: "L105", line: 3, col: 1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestStrayRawStrings(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "gap_between_raw_lines_resumes_valid_entry",
			src:  "@lkal \"0.1\"\na =\n\\\\x\n\n\\\\y\nb =\n[1,\n2]\n",
			want: []expectedError{
				{code: "L104", line: 5, col: 1},
			},
		},
		{
			name: "gap_between_raw_lines_preserves_block_semantic_error",
			src:  "@lkal \"0.1\"\na =\n\\\\x\n\n\\\\y\nb {\nc = 1\nc = 2\n}\n",
			want: []expectedError{
				{code: "L104", line: 5, col: 1},
				{code: "L201", line: 8, col: 1},
			},
		},
		{
			name: "duplicate_key_precedes_raw_gap_error",
			src:  "@lkal \"0.1\"\na = 1\na =\n\\\\x\n\n\\\\y\n",
			want: []expectedError{
				{code: "L201", line: 3, col: 1},
				{code: "L104", line: 6, col: 1},
			},
		},
		{
			name: "top_level_raw_run_without_assign",
			src:  "@lkal \"0.1\"\n\\\\x\n\\\\y\n",
			want: []expectedError{
				{code: "L106", line: 2, col: 1},
			},
		},
		{
			name: "stray_raw_run_after_brace_recovery",
			src:  "@lkal \"0.1\"\na =\n\\\\x\n}\n\n\\\\y\n",
			want: []expectedError{
				{code: "L106", line: 4, col: 1},
				{code: "L106", line: 6, col: 1},
			},
		},
		{
			name: "comma_line_between_raw_values_yields_l106",
			src:  "@lkal \"0.1\"\na =\n\\\\x\n,\n\\\\y\n",
			want: []expectedError{
				{code: "L106", line: 5, col: 1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestBOMRecovery(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "bom_before_head_comment",
			src:  "\ufeff### c\n@lkal \"0.1\"\na = 1\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 1},
			},
		},
		{
			name: "bom_missing_version",
			src:  "\ufeffa = 1\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 1},
				{code: "L105", line: 1, col: 2},
			},
		},
		{
			name: "bom_unsupported_version",
			src:  "\ufeff@lkal \"0.2\"\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 1},
				{code: "L207", line: 1, col: 8},
			},
		},
		{
			name: "bom_empty_document",
			src:  "\ufeff",
			want: []expectedError{
				{code: "L008", line: 1, col: 1},
				{code: "L105", line: 1, col: 2},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestHeaderRecovery(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "missing_version_string_resumes_multiline_entry",
			src:  "@lkal\na =\n[1,\n2]\n",
			want: []expectedError{
				{code: "L105", line: 1, col: 6},
			},
		},
		{
			name: "missing_version_string_resumes_subsequent_entry",
			src:  "@lkal\na = b\n",
			want: []expectedError{
				{code: "L105", line: 1, col: 6},
				{code: "L102", line: 2, col: 5},
			},
		},
		{
			name: "unexpected_brace_on_header_line_recovers_to_next_line",
			src:  "@lkal \"0.1\" {\nb = c\n}\nd = e\n",
			want: []expectedError{
				{code: "L106", line: 1, col: 13},
				{code: "L102", line: 4, col: 5},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestEOFInsideBrackets(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "eof_inside_list",
			src:  "@lkal \"0.1\"\na = [1, \"x\"",
			want: []expectedError{
				{code: "L103", line: 2, col: 5},
			},
		},
		{
			name: "eof_after_equal_in_record",
			src:  "@lkal \"0.1\"\na {\n b =",
			want: []expectedError{
				{code: "L103", line: 2, col: 3},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestUnexpectedTokenHints(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		line     int
		col      int
		wantHint string
	}{
		{
			name:     "entry_start_closing_brace",
			src:      "@lkal \"0.1\"\n}\n",
			line:     2,
			col:      1,
			wantHint: `start an entry with key = value, key { ... } or key "label" { ... }`,
		},
		{
			name:     "entry_start_raw_line",
			src:      "@lkal \"0.1\"\n\\\\x\n",
			line:     2,
			col:      1,
			wantHint: `start an entry with key = value, key { ... } or key "label" { ... }`,
		},
		{
			name:     "keyword_as_key",
			src:      "@lkal \"0.1\"\ntrue = 1\n",
			line:     2,
			col:      1,
			wantHint: "quote a reserved word to use it as a key",
		},
		{
			name:     "after_key",
			src:      "@lkal \"0.1\"\na 1\n",
			line:     2,
			col:      3,
			wantHint: `follow the key with =, { or "label"`,
		},
		{
			name:     "after_label",
			src:      "@lkal \"0.1\"\na \"x\" 1\n",
			line:     2,
			col:      7,
			wantHint: "follow the label with {",
		},
		{
			name:     "after_value",
			src:      "@lkal \"0.1\"\na = 1 b = 2\n",
			line:     2,
			col:      7,
			wantHint: "separate entries with a newline or a comma",
		},
		{
			name:     "brace_after_equal",
			src:      "@lkal \"0.1\"\na = { b = 1 }\n",
			line:     2,
			col:      5,
			wantHint: "write a record without =: key { ... }",
		},
		{
			name:     "nested_list_in_list",
			src:      "@lkal \"0.1\"\na = [[1]]\n",
			line:     2,
			col:      6,
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "record_in_list",
			src:      "@lkal \"0.1\"\na = [{}]\n",
			line:     2,
			col:      6,
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "missing_comma_in_list",
			src:      "@lkal \"0.1\"\na = [1 2]\n",
			line:     2,
			col:      8,
			wantHint: "separate list elements with ,",
		},
		{
			name:     "unexpected_closing_bracket_in_assign",
			src:      "@lkal \"0.1\"\na = ]\n",
			line:     2,
			col:      5,
			wantHint: `write a value: string, number, quantity, boolean, enum, reference, list or \\ lines`,
		},
		{
			name:     "unexpected_equal_in_list",
			src:      "@lkal \"0.1\"\na = [=]\n",
			line:     2,
			col:      6,
			wantHint: `write a value: string, number, quantity, boolean, enum, reference, list or \\ lines`,
		},
		{
			name:     "comment_between_assign_and_value",
			src:      "@lkal \"0.1\"\na =\n### c\n",
			line:     3,
			col:      1,
			wantHint: "move the comment above the entry",
		},
		{
			name:     "version_trailing_character",
			src:      "@lkal \"0.1\" a = 1\n",
			line:     1,
			col:      13,
			wantHint: "end the line after the version declaration",
		},
		{
			name:     "closing_brace_after_top_level_value",
			src:      "@lkal \"0.1\"\na = [1, \"x\"] }\n",
			line:     2,
			col:      14,
			wantHint: "separate entries with a newline or a comma",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertSingleErrorHint(t, tc.name+".lkal", tc.src, "L106", tc.line, tc.col, tc.wantHint)
		})
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

func TestErrorCatalogJSONAndPath(t *testing.T) {
	type errCatCase struct {
		code      string
		wantMsg   string
		wantHint  string
		checkHint bool
	}

	tests := []errCatCase{
		{
			code:      "L301",
			wantMsg:   "invalid JSON",
			wantHint:  "fix the JSON syntax",
			checkHint: true,
		},
		{
			code:      "L302",
			wantMsg:   "null value",
			wantHint:  "omit the key instead of writing null",
			checkHint: true,
		},
		{
			code:      "L303",
			wantMsg:   "number in exponent notation",
			wantHint:  "write the number without an exponent",
			checkHint: true,
		},
		{
			code:      "L304",
			wantMsg:   "list element is not a scalar",
			wantHint:  "use only scalars as list elements",
			checkHint: true,
		},
		{
			code:      "L305",
			wantMsg:   "malformed tag",
			checkHint: false,
		},
		{
			code:      "L401",
			wantMsg:   "malformed path",
			wantHint:  "separate segments with / and quote segments that contain other characters",
			checkHint: true,
		},
		{
			code:      "L402",
			wantMsg:   "path not found",
			checkHint: false,
		},
		{
			code:      "L403",
			wantMsg:   "invalid value",
			checkHint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := newError("", Pos{}, tt.code, "")
			if err == nil {
				t.Fatalf("newError(%q) returned nil", tt.code)
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
	type nestTCatalogCase struct {
		code     string
		wantMsg  string
		wantHint string
	}

	tests := []nestTCatalogCase{
		{
			code:     "L107",
			wantMsg:  "blocks nested too deeply",
			wantHint: "nest blocks at most 1000 levels deep",
		},
	}

	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			err := newError("", Pos{}, tc.code, "")
			if err == nil {
				t.Fatalf("newError(%q) returned nil: got nil, want non-nil *Error", tc.code)
			}
			if err.Code != tc.code {
				t.Errorf("code mismatch for %s: got %q, want %q", tc.code, err.Code, tc.code)
			}
			if err.Msg != tc.wantMsg {
				t.Errorf("msg mismatch for %s: got %q, want %q", tc.code, err.Msg, tc.wantMsg)
			}
			if err.Hint != tc.wantHint {
				t.Errorf("hint mismatch for %s: got %q, want %q", tc.code, err.Hint, tc.wantHint)
			}
		})
	}
}
