package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"strings"
	"testing"
)

func TestSetDepth(t *testing.T) {
	docSrc := "@lkal \"0.1\"\n" + strings.Repeat("r {\n", 999) + "x = 1\n" + strings.Repeat("}\n", 999)
	setPath := strings.Repeat("r/", 999) + "sub"

	tests := []struct {
		name     string
		value    Value
		wantErr  bool
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "innermost_record_add_two_levels_exceeds_1000",
			value:    Record{Fields: []Field{{Key: "a", Value: Record{Fields: []Field{{Key: "b", Value: Int(1)}}}}}},
			wantErr:  true,
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "nest blocks at most 1000 levels deep",
		},
		{
			name:    "innermost_record_add_one_level_succeeds",
			value:   Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "depth.lkal", docSrc)
			origFmtBytes := f.Format()
			if origFmtBytes == nil {
				t.Fatalf("Format() returned nil, want non-nil")
			}
			origFmt := string(origFmtBytes)

			err := f.Set(setPath, tc.value)
			if tc.wantErr {
				pathTCheckSingleError(t, err, "depth.lkal", tc.wantCode, tc.wantMsg, tc.wantHint)

				postFmtBytes := f.Format()
				if postFmtBytes == nil {
					t.Fatalf("Format() returned nil after failed Set, want non-nil")
				}
				postFmt := string(postFmtBytes)
				if postFmt != origFmt {
					t.Errorf("File changed after failed Set:\ngot:\n%s\nwant:\n%s", postFmt, origFmt)
				}
				return
			}

			if err != nil {
				t.Fatalf("Set(%q, %#v) unexpected error: %v", setPath, tc.value, err)
			}
			gotVal, getErr := f.Get(setPath)
			if getErr != nil {
				t.Fatalf("Get(%q) unexpected error: %v", setPath, getErr)
			}
			if gotVal == nil {
				t.Fatalf("Get(%q) returned nil Value", setPath)
			}
			pathTCheckEqualAndOrder(t, gotVal, tc.value)

			postFmtBytes := f.Format()
			if postFmtBytes == nil {
				t.Fatalf("Format() returned nil after successful Set, want non-nil")
			}
			parsed := pathTParse(t, "", string(postFmtBytes))
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())
		})
	}
}

func TestSetDepthOrder(t *testing.T) {
	deep := Record{}
	for i := 0; i < 1000; i++ {
		deep = Record{Fields: []Field{{Key: "x", Value: deep}}}
	}

	tests := []struct {
		name     string
		value    Value
		wantHint string
	}{
		{
			name:     "deep_record_first_exceeds_depth_limit",
			value:    Record{Fields: []Field{{Key: "a", Value: deep}, {Key: "b", Value: Decimal("1.")}}},
			wantHint: "nest blocks at most 1000 levels deep",
		},
		{
			name:     "invalid_decimal_first_fails_decimal_validation",
			value:    Record{Fields: []Field{{Key: "b", Value: Decimal("1.")}, {Key: "a", Value: deep}}},
			wantHint: "write a decimal such as 0.25",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", "@lkal \"0.1\"\n")
			if f == nil {
				t.Fatalf("pathTParse returned nil File: got nil, want non-nil")
			}
			origFmtBytes := f.Format()
			if origFmtBytes == nil {
				t.Fatalf("Format() returned nil: got nil, want non-nil")
			}
			origFmt := string(origFmtBytes)

			err := f.Set("r", tc.value)
			if err == nil {
				t.Fatalf("Set(%q) error: got nil, want error", "r")
			}
			pathTCheckSingleError(t, err, "test.lkal", "L403", "invalid value", tc.wantHint)

			postFmtBytes := f.Format()
			if postFmtBytes == nil {
				t.Fatalf("Format() returned nil after failed Set: got nil, want non-nil")
			}
			postFmt := string(postFmtBytes)
			if postFmt != origFmt {
				t.Errorf("File changed after failed Set:\ngot:\n%s\nwant:\n%s", postFmt, origFmt)
			}
		})
	}
}

func TestParseValueImplementationReview(t *testing.T) {
	deepSrc1001 := "{ " + strings.Repeat("a { ", 1000) + "x = 1" + strings.Repeat(" }", 1001)
	col1001 := strings.LastIndex(deepSrc1001, "{") + 1
	if col1001 != 4001 {
		t.Fatalf("computed col1001 mismatch: got %d, want %d", col1001, 4001)
	}

	errorTests := []struct {
		name       string
		src        string
		wantErrors []pathTExpectedError
	}{
		{
			name: "list_mixed_types_with_extra_token",
			src:  `[1, "a"] x`,
			wantErrors: []pathTExpectedError{
				{
					code: "L204",
					line: 1,
					col:  5,
					msg:  "list mixes types",
					hint: "use one type for all elements",
				},
				{
					code: "L106",
					line: 1,
					col:  10,
					msg:  "unexpected token",
					hint: "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines",
				},
			},
		},
		{
			name: "record_dollar_key_with_extra_token",
			src:  `{ "$k" = 1 } junk`,
			wantErrors: []pathTExpectedError{
				{
					code: "L205",
					line: 1,
					col:  3,
					msg:  "key starts with $",
					hint: "rename the key",
				},
				{
					code: "L106",
					line: 1,
					col:  14,
					msg:  "unexpected token",
					hint: "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines",
				},
			},
		},
		{
			name: "list_syntax_error_no_extra_token_l106",
			src:  "[\n### c\n1] x",
			wantErrors: []pathTExpectedError{
				{
					code: "L002",
					line: 2,
					col:  1,
					msg:  "comment at the end of a line or inside a list",
					hint: "move the comment to its own line outside the list",
				},
			},
		},
		{
			name: "record_value_nested_1001_levels_l107",
			src:  deepSrc1001,
			wantErrors: []pathTExpectedError{
				{
					code: "L107",
					line: 1,
					col:  col1001,
					msg:  "blocks nested too deeply",
					hint: "nest blocks at most 1000 levels deep",
				},
			},
		},
	}

	for _, tc := range errorTests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ParseValue([]byte(tc.src))
			if err == nil {
				t.Fatalf("ParseValue(%q) succeeded unexpectedly, got %#v", tc.src, val)
			}
			if val != nil {
				t.Errorf("ParseValue(%q) value:\ngot:  %#v\nwant: nil", tc.src, val)
			}
			pathTCheckErrors(t, err, tc.wantErrors)
		})
	}

	t.Run("record_value_nested_1000_levels_succeeds", func(t *testing.T) {
		deepSrc1000 := "{ " + strings.Repeat("a { ", 999) + "x = 1" + strings.Repeat(" }", 1000)
		val, err := ParseValue([]byte(deepSrc1000))
		if err != nil {
			t.Fatalf("ParseValue(1000 levels) unexpected error: %v", err)
		}
		if val == nil {
			t.Fatalf("ParseValue(1000 levels) returned nil Value, want Record")
		}
		if _, ok := val.(Record); !ok {
			t.Errorf("ParseValue(1000 levels) type:\ngot:  %T\nwant: Record", val)
		}
	})
}
