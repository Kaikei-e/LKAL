package lkaljson_test

import "strings"

var jsonTSemanticErrCases = []jsonTErrCase{
	{
		name:       "L005_int64_overflow",
		input:      `{"$lkal": "0.1", "a": 9223372036854775808}`,
		wantErrors: []string{"L005 1:23"},
	},
	{
		name:       "L005_101_digit_integer",
		input:      `{"$lkal": "0.1", "a": ` + strings.Repeat("9", 101) + `}`,
		wantErrors: []string{"L005 1:23"},
	},
	{
		name:       "L005_101_digit_decimal",
		input:      `{"$lkal": "0.1", "a": ` + strings.Repeat("1", 50) + "." + strings.Repeat("2", 51) + `}`,
		wantErrors: []string{"L005 1:23"},
	},
	{
		name:       "L005_101_digit_qty",
		input:      `{"$lkal": "0.1", "a": {"$qty": "` + strings.Repeat("9", 101) + `Mi"}}`,
		wantErrors: []string{"L005 1:32"},
	},
	{
		name:       "L105_top_level_array",
		input:      `[]`,
		wantErrors: []string{"L105 1:1"},
	},
	{
		name:       "L105_top_level_string",
		input:      `"hello"`,
		wantErrors: []string{"L105 1:1"},
	},
	{
		name:       "L105_top_level_number",
		input:      `123`,
		wantErrors: []string{"L105 1:1"},
	},
	{
		name:       "L105_first_key_not_lkal",
		input:      `{"a": 1}`,
		wantErrors: []string{"L105 1:2"},
	},
	{
		name:       "L105_empty_object",
		input:      `{}`,
		wantErrors: []string{"L105 1:2"},
	},
	{
		name:       "L105_lkal_not_string",
		input:      `{"$lkal": 1}`,
		wantErrors: []string{"L105 1:11"},
	},
	{
		name:       "L105_repeated_lkal",
		input:      `{"$lkal": "0.1", "$lkal": "0.1"}`,
		wantErrors: []string{"L105 1:18"},
	},
	{
		name:       "L105_first_key_not_lkal_then_null",
		input:      `{"a": null}`,
		wantErrors: []string{"L105 1:2", "L302 1:7"},
	},
	{
		name:       "L207_unsupported_version_02",
		input:      `{"$lkal": "0.2"}`,
		wantErrors: []string{"L207 1:11"},
	},
	{
		name:       "L207_unsupported_version_010",
		input:      `{"$lkal": "0.10"}`,
		wantErrors: []string{"L207 1:11"},
	},
	{
		name:       "L201_duplicate_key",
		input:      `{"$lkal": "0.1", "a": 1, "a": 2}`,
		wantErrors: []string{"L201 1:26"},
	},
	{
		name:       "L201_duplicate_label_in_coll",
		input:      `{"$lkal": "0.1", "c": {"$coll": {"x": {}, "x": {}}}}`,
		wantErrors: []string{"L201 1:43"},
	},
	{
		name:       "L204_mixed_list",
		input:      `{"$lkal": "0.1", "a": [1, "two"]}`,
		wantErrors: []string{"L204 1:27"},
	},
	{
		name:       "L204_int_and_decimal",
		input:      `{"$lkal": "0.1", "a": [1, 1.5]}`,
		wantErrors: []string{"L204 1:27"},
	},
	{
		name:       "L204_suppressed_after_null",
		input:      `{"$lkal": "0.1", "a": [1, null]}`,
		wantErrors: []string{"L302 1:27"},
	},
	{
		name:       "L204_suppressed_after_l303",
		input:      `{"$lkal": "0.1", "a": [1, 1e5]}`,
		wantErrors: []string{"L303 1:27"},
	},
	{
		name:       "L204_suppressed_after_l304",
		input:      `{"$lkal": "0.1", "a": [1, []]}`,
		wantErrors: []string{"L304 1:27"},
	},
	{
		name:       "L204_suppressed_after_l305",
		input:      `{"$lkal": "0.1", "a": [1, {"$enum": 123}]}`,
		wantErrors: []string{"L305 1:37"},
	},
	{
		name:       "L205_dollar_key_non_first",
		input:      `{"$lkal": "0.1", "a": 1, "$extra": 2}`,
		wantErrors: []string{"L205 1:26"},
	},
	{
		name:       "L205_nested_record_first_dollar_key",
		input:      `{"$lkal": "0.1", "rec": {"$schema": 1}}`,
		wantErrors: []string{"L205 1:26"},
	},
	{
		name:       "L205_tag_name_in_non_first_position",
		input:      `{"$lkal": "0.1", "a": 1, "$enum": "tcp"}`,
		wantErrors: []string{"L205 1:26"},
	},
	{
		name:       "L205_dollar_key_precedes_l201_duplicate",
		input:      `{"$lkal": "0.1", "$a": 1, "$a": 2}`,
		wantErrors: []string{"L205 1:18", "L205 1:27"},
	},
	{
		name:       "L205_coll_member_nested_schema_key",
		input:      `{"$lkal": "0.1", "c": {"$coll": {"x": {"$schema": 1}}}}`,
		wantErrors: []string{"L205 1:40"},
	},
	{
		name:       "several_errors_in_order",
		input:      `{"$lkal": "0.1", "a": 1e5, "b": null}`,
		wantErrors: []string{"L303 1:23", "L302 1:33"},
	},
	{
		name:       "errors_before_syntax_error_kept",
		input:      `{"$lkal": "0.1", "a": null, "b": }`,
		wantErrors: []string{"L302 1:23", "L301 1:34"},
	},
	{
		name:       "multiline_and_unicode_column_count",
		input:      "{\n  \"$lkal\": \"0.1\",\n  \"名前\": null\n}",
		wantErrors: []string{"L302 3:9"},
	},
	{
		name:        "error_cap_25_nulls",
		input:       `{"$lkal": "0.1", "a": [null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null]}`,
		wantCount20: true,
	},
	{
		name:       "L301_escape_unknown_char",
		input:      `{"a":"\q"}`,
		wantErrors: []string{"L105 1:2", "L301 1:7"},
	},
	{
		name:       "L301_escape_hex_cutoff",
		input:      `{"a":"\u1x`,
		wantErrors: []string{"L105 1:2", "L301 1:7"},
	},
	{
		name:       "L301_escape_unpaired_high_surrogate",
		input:      `{"a":"\uD800"}`,
		wantErrors: []string{"L105 1:2", "L301 1:7"},
	},
	{
		name:       "L301_escape_surrogate_cutoff",
		input:      `{"a":"\uD800`,
		wantErrors: []string{"L105 1:2", "L301 1:7"},
	},
	{
		name:       "L301_escape_surrogate_bad_low_hex",
		input:      `{"a":"\uD800\uDC0G"}`,
		wantErrors: []string{"L105 1:2", "L301 1:7"},
	},
	{
		name:       "L301_escape_unpaired_low_surrogate",
		input:      `{"a":"\uDC00"}`,
		wantErrors: []string{"L105 1:2", "L301 1:7"},
	},
	{
		name:       "L301_invalid_utf8_in_string",
		input:      "{\"a\":\"x\xffy\"}",
		wantErrors: []string{"L105 1:2", "L301 1:8"},
	},
	{
		name:       "L301_utf8_sequence_cutoff_at_end",
		input:      "{\"a\":\"\xe6\x97",
		wantErrors: []string{"L105 1:2", "L301 1:7"},
	},
	{
		name:       "L301_trailing_comma_in_array",
		input:      `[1,]`,
		wantErrors: []string{"L105 1:1", "L301 1:3"},
	},
	{
		name:       "L301_leading_comma_in_array",
		input:      `[,1]`,
		wantErrors: []string{"L105 1:1", "L301 1:2"},
	},
	{
		name:       "L301_consecutive_commas_in_object",
		input:      `{"a":1,,}`,
		wantErrors: []string{"L105 1:2", "L301 1:8"},
	},
	{
		name:       "L301_partial_literal_tru_at_end",
		input:      `{"a":tru`,
		wantErrors: []string{"L105 1:2", "L301 1:9"},
	},
	{
		name:       "L301_partial_literal_tRue",
		input:      `{"a":tRue}`,
		wantErrors: []string{"L105 1:2", "L301 1:7"},
	},
	{
		name:       "L301_partial_literal_nul_before_brace",
		input:      `{"a":nul}`,
		wantErrors: []string{"L105 1:2", "L301 1:9"},
	},
	{
		name:       "L301_unterminated_string_at_end",
		input:      `{"a":"abc`,
		wantErrors: []string{"L105 1:2", "L301 1:10"},
	},
	{
		name:       "L301_top_level_invalid_char_x",
		input:      `x`,
		wantErrors: []string{"L301 1:1"},
	},
	{
		name:       "L301_top_level_invalid_char_close_brace",
		input:      `}`,
		wantErrors: []string{"L301 1:1"},
	},
	{
		name:       "L105_L205_first_key_dollar_x",
		input:      `{"$x": null}`,
		wantErrors: []string{"L105 1:2", "L205 1:2"},
	},
	{
		name:       "L105_misplaced_lkal_after_data",
		input:      `{"a": 1, "$lkal": "0.1"}`,
		wantErrors: []string{"L105 1:2", "L105 1:10"},
	},
	{
		name:       "L105_top_level_null",
		input:      `null`,
		wantErrors: []string{"L105 1:1"},
	},
	{
		name:       "L105_lkal_value_null",
		input:      `{"$lkal": null}`,
		wantErrors: []string{"L105 1:11"},
	},
	{
		name:       "L302_coll_nested_null_members",
		input:      `{"$lkal":"0.1","c":{"$coll":{"e1":{"x":null,"y":null},"e2":{"z":null}}}}`,
		wantErrors: []string{"L302 1:40", "L302 1:49", "L302 1:65"},
	},
	{
		name:       "L305_L301_enum_array_with_syntax_error",
		input:      `{"$lkal":"0.1","a":{"$enum":[1,}}`,
		wantErrors: []string{"L305 1:29", "L301 1:31"},
	},
	{
		name:       "L105_L301_lkal_array_with_syntax_error",
		input:      `{"$lkal": [1,}`,
		wantErrors: []string{"L105 1:11", "L301 1:13"},
	},
	{
		name:       "amendment4_unclosed_lkal_key",
		input:      `{"$lkal"`,
		wantErrors: []string{"L301 1:9"},
	},
	{
		name:       "amendment4_missing_colon_after_lkal",
		input:      `{"$lkal" "0.1"}`,
		wantErrors: []string{"L301 1:10"},
	},
	{
		name:       "amendment4_lkal_partial_boolean",
		input:      `{"$lkal": tru}`,
		wantErrors: []string{"L301 1:14"},
	},
	{
		name:       "amendment4_lkal_unquoted_char",
		input:      `{"$lkal": x}`,
		wantErrors: []string{"L301 1:11"},
	},
	{
		name:       "amendment4_enum_partial_boolean",
		input:      `{"$lkal":"0.1","a":{"$enum":tru}}`,
		wantErrors: []string{"L301 1:32"},
	},
	{
		name:       "amendment4_enum_empty_value",
		input:      `{"$lkal":"0.1","a":{"$enum":}`,
		wantErrors: []string{"L301 1:29"},
	},
	{
		name:       "amendment4_coll_partial_boolean",
		input:      `{"$lkal":"0.1","c":{"$coll":tru}}`,
		wantErrors: []string{"L301 1:32"},
	},
	{
		name:       "amendment4_coll_member_unclosed_string",
		input:      `{"$lkal":"0.1","c":{"$coll":{"x":"abc}}}`,
		wantErrors: []string{"L301 1:41"},
	},
	{
		name:       "amendment4_coll_record_error_allows_second_key",
		input:      `{"$lkal":"0.1","c":{"$coll":{"x":{"k":null}},"y":1}}`,
		wantErrors: []string{"L302 1:39", "L305 1:46"},
	},
	{
		name:       "amendment5_max_depth_key_syntax_error",
		input:      `{"$lkal":"0.1","max depth":tru}`,
		wantErrors: []string{"L301 1:31"},
		wantHint:   "fix the JSON syntax",
	},
	{
		name:       "amendment5_coll_member_error_null",
		input:      `{"$lkal":"0.1","c":{"$coll":{"a":1,"b":{"k":null}}}}`,
		wantErrors: []string{"L305 1:34", "L302 1:45"},
	},
	{
		name:       "amendment5_coll_member_error_duplicate_label",
		input:      `{"$lkal":"0.1","c":{"$coll":{"a":1,"a":{}}}}`,
		wantErrors: []string{"L305 1:34", "L201 1:36"},
	},
	{
		name:       "amendment5_coll_member_error_dollar_key",
		input:      `{"$lkal":"0.1","c":{"$coll":{"a":{"$enum":"x"},"b":{"$k":1}}}}`,
		wantErrors: []string{"L305 1:34", "L205 1:53"},
	},
}
