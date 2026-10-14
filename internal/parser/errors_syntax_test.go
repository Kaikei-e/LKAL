package parser_test

import . "github.com/Kaikei-e/LKAL"

import "testing"

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
