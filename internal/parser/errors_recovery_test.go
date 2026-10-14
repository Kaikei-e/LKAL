package parser_test

import . "github.com/Kaikei-e/LKAL"

import "testing"

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
			{code: "L202", line: 3, col: 1},
			{code: "L202", line: 4, col: 1},
		})
	})

	t.Run("record_element_record", func(t *testing.T) {
		src := `@lkal "0.1"
a {}
a "x" {}
a {}
`
		assertExactErrors(t, "kind_mixing_record_elem_record.lkal", src, []expectedError{
			{code: "L202", line: 3, col: 1},
			{code: "L202", line: 4, col: 1},
		})
	})
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
