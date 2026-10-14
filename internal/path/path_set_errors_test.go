package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetErrors(t *testing.T) {
	baseSrc := `@lkal "0.1"

a = 1
service "web" {
  replicas = 3
}
`
	tests := []struct {
		name     string
		src      string
		path     string
		value    Value
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "l401_empty_path",
			src:      baseSrc,
			path:     "",
			value:    Int(1),
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: "separate segments with / and quote segments that contain other characters",
		},
		{
			name:     "l401_leading_slash",
			src:      baseSrc,
			path:     "/a",
			value:    Int(1),
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: "separate segments with / and quote segments that contain other characters",
		},
		{
			name:     "l401_space_in_segment",
			src:      baseSrc,
			path:     "a b",
			value:    Int(1),
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: "separate segments with / and quote segments that contain other characters",
		},
		{
			name:     "l401_unquoted_dollar_in_path",
			src:      baseSrc,
			path:     "$bad",
			value:    Int(1),
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: "separate segments with / and quote segments that contain other characters",
		},
		{
			name:     "l402_missing_parent",
			src:      baseSrc,
			path:     "missing/key",
			value:    Int(1),
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"missing\"",
		},
		{
			name:     "l402_parent_is_assignment",
			src:      baseSrc,
			path:     "service/web/replicas/key",
			value:    Int(1),
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"key\"",
		},
		{
			name:     "l403_nil_top_level",
			src:      baseSrc,
			path:     "a",
			value:    nil,
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a non-nil value",
		},
		{
			name:     "l403_nil_list_element",
			src:      baseSrc,
			path:     "list",
			value:    List{nil},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a non-nil value",
		},
		{
			name:     "l403_nil_field_value",
			src:      baseSrc,
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "k", Value: nil}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a non-nil value",
		},
		{
			name:     "l403_bad_decimal_trailing_dot",
			src:      baseSrc,
			path:     "dec",
			value:    Decimal("1."),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a decimal such as 0.25",
		},
		{
			name:     "l403_bad_decimal_no_integer",
			src:      baseSrc,
			path:     "dec",
			value:    Decimal(".5"),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a decimal such as 0.25",
		},
		{
			name:     "l403_bad_quantity_empty_unit",
			src:      baseSrc,
			path:     "qty",
			value:    Quantity{Number: "1", Unit: ""},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a quantity such as 512Mi",
		},
		{
			name:     "l403_bad_quantity_invalid_number",
			src:      baseSrc,
			path:     "qty",
			value:    Quantity{Number: "x", Unit: "Mi"},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a quantity such as 512Mi",
		},
		{
			name:     "l403_101_digit_decimal",
			src:      baseSrc,
			path:     "dec",
			value:    Decimal("1." + strings.Repeat("0", 100)),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "keep numbers within 100 digits",
		},
		{
			name:     "l403_bad_enum_leading_digit",
			src:      baseSrc,
			path:     "e",
			value:    Enum("1x"),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write an enum name that starts with a letter or _",
		},
		{
			name:     "l403_ref_with_no_segments",
			src:      baseSrc,
			path:     "r",
			value:    Ref{},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a reference with at least one segment",
		},
		{
			name:     "l403_invalid_utf8_string",
			src:      baseSrc,
			path:     "s",
			value:    String("\xff"),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use valid UTF-8 text",
		},
		{
			name:     "l403_invalid_utf8_key",
			src:      baseSrc,
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "k\xff", Value: Int(1)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use valid UTF-8 text",
		},
		{
			name:     "l403_invalid_utf8_label",
			src:      baseSrc,
			path:     "coll",
			value:    Collection{Elems: []Elem{{Label: "l\xff", Body: Record{Fields: []Field{{Key: "k", Value: Int(1)}}}}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use valid UTF-8 text",
		},
		{
			name:     "l403_invalid_utf8_ref_segment",
			src:      baseSrc,
			path:     "r",
			value:    Ref{"s\xff"},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use valid UTF-8 text",
		},
		{
			name:     "l403_list_inside_list",
			src:      baseSrc,
			path:     "list",
			value:    List{List{Int(1)}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "l403_record_inside_list",
			src:      baseSrc,
			path:     "list",
			value:    List{Record{}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "l403_collection_inside_list",
			src:      baseSrc,
			path:     "list",
			value:    List{Collection{Elems: []Elem{{Label: "a", Body: Record{}}}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "l403_mixed_list",
			src:      baseSrc,
			path:     "list",
			value:    List{Int(1), String("two")},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use one type for all list elements",
		},
		{
			name:     "l403_dollar_key_in_value",
			src:      baseSrc,
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "$bad", Value: Int(1)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "rename keys that start with $",
		},
		{
			name:     "l403_dollar_key_as_last_path_segment",
			src:      baseSrc,
			path:     "\"$bad\"",
			value:    Int(1),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "rename keys that start with $",
		},
		{
			name:     "l403_repeated_key",
			src:      baseSrc,
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "dup", Value: Int(1)}, {Key: "dup", Value: Int(2)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "remove the duplicate key",
		},
		{
			name:     "l403_empty_collection",
			src:      baseSrc,
			path:     "coll",
			value:    Collection{Elems: nil},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write at least one collection element",
		},
		{
			name:     "l403_repeated_label",
			src:      baseSrc,
			path:     "coll",
			value:    Collection{Elems: []Elem{{Label: "dup", Body: Record{}}, {Label: "dup", Body: Record{}}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "remove the duplicate label",
		},
		{
			name:     "l403_non_record_at_element_path",
			src:      baseSrc,
			path:     "service/newelem",
			value:    Int(1),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a record for a collection element",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", tc.src)
			origFormat := f.Format()
			err := f.Set(tc.path, tc.value)
			if err == nil {
				t.Fatalf("Set(%q, ...) succeeded unexpectedly, want error", tc.path)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("expected exactly 1 error, got %d: %v", len(errList.Errors), errList)
			}
			e := errList.Errors[0]
			if e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", e.Code, tc.wantCode)
			}
			if e.Pos != (Pos{}) {
				t.Errorf("Pos = %v, want zero Pos", e.Pos)
			}
			if e.File != "test.lkal" {
				t.Errorf("File = %q, want %q", e.File, "test.lkal")
			}
			if e.Msg != tc.wantMsg {
				t.Errorf("Msg = %q, want %q", e.Msg, tc.wantMsg)
			}
			if e.Hint != tc.wantHint {
				t.Errorf("Hint = %q, want %q", e.Hint, tc.wantHint)
			}
			if !bytes.Equal(f.Format(), origFormat) {
				t.Errorf("File format changed after failed Set")
			}
		})
	}
}
