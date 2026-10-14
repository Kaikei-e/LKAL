package lkaljson_test

import . "github.com/Kaikei-e/LKAL"

import (
	"reflect"
	"testing"
)

type jsonTErrCase struct {
	name        string
	input       string
	wantErrors  []string
	wantHint    string
	wantCount20 bool
}

var jsonTSyntaxAndTagErrCases = []jsonTErrCase{
	{
		name:       "L301_contract_f_ex1_empty_value",
		input:      `{"$lkal": "0.1", "a": }`,
		wantErrors: []string{"L301 1:23"},
	},
	{
		name:       "L301_contract_f_ex2_unclosed_object",
		input:      `{"$lkal": "0.1", "a": 1`,
		wantErrors: []string{"L301 1:24"},
	},
	{
		name:       "L301_contract_f_ex3_trailing_char",
		input:      `{"$lkal": "0.1"} x`,
		wantErrors: []string{"L301 1:18"},
	},
	{
		name:       "L301_contract_f_ex4_leading_zero",
		input:      `{"$lkal": "0.1", "a": 01}`,
		wantErrors: []string{"L301 1:24"},
	},
	{
		name:       "L301_contract_f_ex5_empty_input",
		input:      ``,
		wantErrors: []string{"L301 1:1"},
	},
	{
		name:       "L301_trailing_comma",
		input:      `{"$lkal": "0.1", "a": 1,}`,
		wantErrors: []string{"L301 1:24"},
	},
	{
		name:       "L301_invalid_utf8_byte",
		input:      "{\"$lkal\": \"0.1\", \"a\": \"\xff\"}",
		wantErrors: []string{"L301 1:24"},
	},
	{
		name:       "L301_utf8_bom",
		input:      "\xEF\xBB\xBF{\"$lkal\": \"0.1\"}",
		wantErrors: []string{"L301 1:1"},
	},
	{
		name:       "L301_unpaired_surrogate_escape",
		input:      `{"$lkal": "0.1", "a": "\uD800"}`,
		wantErrors: []string{"L301 1:24"},
	},
	{
		name:       "L301_raw_control_char_in_string",
		input:      "{\"$lkal\": \"0.1\", \"a\": \"x\x01y\"}",
		wantErrors: []string{"L301 1:25"},
	},
	{
		name:       "L301_truncated_string",
		input:      "{\"$lkal\": \"0.1\", \"a\": \"unclosed",
		wantErrors: []string{"L301 1:32"},
	},
	{
		name:       "L301_end_of_input_with_lf",
		input:      "{\"$lkal\": \"0.1\",\n",
		wantErrors: []string{"L301 2:1"},
	},
	{
		name:       "L301_whitespace_only",
		input:      "  \n ",
		wantErrors: []string{"L301 2:2"},
	},
	{
		name:       "L301_multiline_line3_syntax_error",
		input:      "{\n  \"$lkal\": \"0.1\",\n  \"a\": }\n",
		wantErrors: []string{"L301 3:8"},
	},
	{
		name:       "L302_member_value_null",
		input:      `{"$lkal": "0.1", "a": null}`,
		wantErrors: []string{"L302 1:23"},
	},
	{
		name:       "L302_array_element_null",
		input:      `{"$lkal": "0.1", "a": [null]}`,
		wantErrors: []string{"L302 1:24"},
	},
	{
		name:       "L302_tag_payload_null",
		input:      `{"$lkal": "0.1", "a": {"$enum": null}}`,
		wantErrors: []string{"L302 1:33"},
	},
	{
		name:       "L302_coll_member_null",
		input:      `{"$lkal": "0.1", "c": {"$coll": {"x": null}}}`,
		wantErrors: []string{"L302 1:39"},
	},
	{
		name:       "L303_exp_lowercase",
		input:      `{"$lkal": "0.1", "a": 1e5}`,
		wantErrors: []string{"L303 1:23"},
	},
	{
		name:       "L303_exp_uppercase_negative",
		input:      `{"$lkal": "0.1", "a": 1E-2}`,
		wantErrors: []string{"L303 1:23"},
	},
	{
		name:       "L303_exp_decimal",
		input:      `{"$lkal": "0.1", "a": 1.5e3}`,
		wantErrors: []string{"L303 1:23"},
	},
	{
		name:       "L303_exp_inside_array",
		input:      `{"$lkal": "0.1", "a": [1e5]}`,
		wantErrors: []string{"L303 1:24"},
	},
	{
		name:       "L304_nested_array_in_list",
		input:      `{"$lkal": "0.1", "a": [[]]}`,
		wantErrors: []string{"L304 1:24"},
	},
	{
		name:       "L304_plain_object_in_list",
		input:      `{"$lkal": "0.1", "a": [{"x": 1}]}`,
		wantErrors: []string{"L304 1:24"},
	},
	{
		name:       "L304_empty_object_in_list",
		input:      `{"$lkal": "0.1", "a": [{}]}`,
		wantErrors: []string{"L304 1:24"},
	},
	{
		name:       "L304_coll_object_in_list",
		input:      `{"$lkal": "0.1", "a": [{"$coll": {"x": {}}}]}`,
		wantErrors: []string{"L304 1:24"},
	},
	{
		name:       "L304_unknown_tag_in_list",
		input:      `{"$lkal": "0.1", "a": [{"$foo": 1}]}`,
		wantErrors: []string{"L304 1:24"},
	},
	{
		name:       "L304_dropped_nested_null_not_checked",
		input:      `{"$lkal": "0.1", "a": [[null]]}`,
		wantErrors: []string{"L304 1:24"},
	},
	{
		name:       "L305_enum_payload_not_string",
		input:      `{"$lkal": "0.1", "a": {"$enum": 123}}`,
		wantErrors: []string{"L305 1:33"},
	},
	{
		name:       "L305_enum_empty_string",
		input:      `{"$lkal": "0.1", "a": {"$enum": ""}}`,
		wantErrors: []string{"L305 1:33"},
	},
	{
		name:       "L305_bad_enum_name",
		input:      `{"$lkal": "0.1", "a": {"$enum": "1bad"}}`,
		wantErrors: []string{"L305 1:33"},
	},
	{
		name:       "L305_qty_payload_not_string",
		input:      `{"$lkal": "0.1", "a": {"$qty": 123}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_qty_no_unit",
		input:      `{"$lkal": "0.1", "a": {"$qty": "512"}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_qty_leading_zero",
		input:      `{"$lkal": "0.1", "a": {"$qty": "01Mi"}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_qty_trailing_dot",
		input:      `{"$lkal": "0.1", "a": {"$qty": "1.Mi"}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_qty_leading_space",
		input:      `{"$lkal": "0.1", "a": {"$qty": " 1Mi"}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_qty_digits_after_unit",
		input:      `{"$lkal": "0.1", "a": {"$qty": "1h30m"}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_ref_payload_not_string",
		input:      `{"$lkal": "0.1", "a": {"$ref": 123}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_ref_empty_string",
		input:      `{"$lkal": "0.1", "a": {"$ref": ""}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_ref_trailing_slash",
		input:      `{"$lkal": "0.1", "a": {"$ref": "a/"}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_ref_bad_escape",
		input:      `{"$lkal": "0.1", "a": {"$ref": "\"\\q\""}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_malformed_ref",
		input:      `{"$lkal": "0.1", "a": {"$ref": "a//b"}}`,
		wantErrors: []string{"L305 1:32"},
	},
	{
		name:       "L305_second_member_in_tag",
		input:      `{"$lkal": "0.1", "a": {"$enum": "tcp", "extra": 1}}`,
		wantErrors: []string{"L305 1:40"},
	},
	{
		name:       "L305_second_tag_key_in_tag_object",
		input:      `{"$lkal": "0.1", "a": {"$enum": "a", "$qty": "1m"}}`,
		wantErrors: []string{"L305 1:38"},
	},
	{
		name:       "L305_dropped_tag_extra_not_checked",
		input:      `{"$lkal": "0.1", "a": {"$enum": 1, "x": null}}`,
		wantErrors: []string{"L305 1:33"},
	},
	{
		name:       "L305_coll_payload_not_object",
		input:      `{"$lkal": "0.1", "c": {"$coll": []}}`,
		wantErrors: []string{"L305 1:33"},
	},
	{
		name:       "L305_coll_empty",
		input:      `{"$lkal": "0.1", "c": {"$coll": {}}}`,
		wantErrors: []string{"L305 1:33"},
	},
	{
		name:       "L305_coll_member_not_object",
		input:      `{"$lkal": "0.1", "c": {"$coll": {"x": 1}}}`,
		wantErrors: []string{"L305 1:39"},
	},
	{
		name:       "L305_coll_member_tag_object",
		input:      `{"$lkal": "0.1", "c": {"$coll": {"x": {"$enum": "tcp"}}}}`,
		wantErrors: []string{"L305 1:39"},
	},
}

func TestFromJSONErrors(t *testing.T) {
	tests := append(append([]jsonTErrCase(nil), jsonTSyntaxAndTagErrCases...), jsonTSemanticErrCases...)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := FromJSON([]byte(tt.input))
			if err == nil {
				t.Fatalf("expected error from FromJSON: got nil, want error, file: %v", f)
			}
			if f != nil {
				t.Errorf("expected nil File on error: got %v, want nil", f)
			}

			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList: got %T (%v), want *ErrorList", err, err)
			}

			got := jsonTExtractErrors(errList)
			if tt.wantCount20 {
				if len(got) != 20 {
					t.Fatalf("expected exactly 20 errors, got %d: %v", len(got), got)
				}
				for i, e := range errList.Errors {
					if e == nil {
						t.Fatalf("expected non-nil error at index %d", i)
					}
					if e.Code != "L302" {
						t.Errorf("expected L302 at error[%d], got %s", i, e.Code)
					}
				}
			} else {
				if !reflect.DeepEqual(got, tt.wantErrors) {
					t.Errorf("error mismatch:\ngot:  %v\nwant: %v", got, tt.wantErrors)
				}
			}
			if tt.wantHint != "" {
				if errList == nil || len(errList.Errors) == 0 {
					t.Fatalf("expected at least 1 error for hint check: got 0, want >= 1")
				}
				first := errList.Errors[0]
				if first == nil {
					t.Fatalf("first error is nil: got nil, want non-nil Error")
				}
				if gotHint, wantHint := first.Hint, tt.wantHint; gotHint != wantHint {
					t.Errorf("hint mismatch: got %q, want %q", gotHint, wantHint)
				}
			}
		})
	}
}
