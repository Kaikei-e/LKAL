package lkaljson_test

import . "github.com/Kaikei-e/LKAL"

import "testing"

func TestFromJSONMaxDepthKeySyntaxErrorHint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode string
		wantHint string
	}{
		{
			name:     "key_containing_max_depth_keeps_default_syntax_hint",
			input:    `{"$lkal":"0.1","max depth":tru}`,
			wantCode: "L301",
			wantHint: "fix the JSON syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := FromJSON([]byte(tt.input))
			if err == nil {
				t.Fatalf("FromJSON(%q) error: got nil, want non-nil error, file: %v", tt.input, f)
			}
			if f != nil {
				t.Errorf("FromJSON(%q) file: got %v, want nil", tt.input, f)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("FromJSON(%q) error type: got %T (%v), want *ErrorList", tt.input, err, err)
			}
			if errList == nil || len(errList.Errors) == 0 {
				t.Fatalf("FromJSON(%q) error count: got 0, want >= 1", tt.input)
			}
			first := errList.Errors[0]
			if first == nil {
				t.Fatalf("FromJSON(%q) first error is nil: got nil, want non-nil Error", tt.input)
			}
			if gotCode, wantCode := first.Code, tt.wantCode; gotCode != wantCode {
				t.Errorf("Code mismatch: got %q, want %q", gotCode, wantCode)
			}
			if gotHint, wantHint := first.Hint, tt.wantHint; gotHint != wantHint {
				t.Errorf("Hint mismatch: got %q, want %q", gotHint, wantHint)
			}
		})
	}
}

func TestFromJSONErrorDetails(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "L301_details",
			input:    `{"$lkal": "0.1", "a": }`,
			wantCode: "L301",
			wantMsg:  "invalid JSON",
			wantHint: "fix the JSON syntax",
		},
		{
			name:     "L302_details",
			input:    `{"$lkal": "0.1", "a": null}`,
			wantCode: "L302",
			wantMsg:  "null value",
			wantHint: "omit the key instead of writing null",
		},
		{
			name:     "L303_details",
			input:    `{"$lkal": "0.1", "a": 1e5}`,
			wantCode: "L303",
			wantMsg:  "number in exponent notation",
			wantHint: "write the number without an exponent",
		},
		{
			name:     "L304_details",
			input:    `{"$lkal": "0.1", "a": [[]]}`,
			wantCode: "L304",
			wantMsg:  "list element is not a scalar",
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "L305_second_member_hint",
			input:    `{"$lkal": "0.1", "a": {"$enum": "tcp", "extra": 1}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: "write a tag object with only one key",
		},
		{
			name:     "L305_enum_hint",
			input:    `{"$lkal": "0.1", "a": {"$enum": 123}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: `write the enum name as a string, for example {"$enum": "tcp"}`,
		},
		{
			name:     "L305_qty_hint",
			input:    `{"$lkal": "0.1", "a": {"$qty": "512"}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: `write the quantity as a string, for example {"$qty": "512Mi"}`,
		},
		{
			name:     "L305_ref_hint",
			input:    `{"$lkal": "0.1", "a": {"$ref": "a//b"}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: `write the reference as a string, for example {"$ref": "secret/db-creds"}`,
		},
		{
			name:     "L305_coll_hint",
			input:    `{"$lkal": "0.1", "c": {"$coll": {}}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: `write {"$coll": {"label": {...}}} with at least one element`,
		},
		{
			name:     "L105_json_hint",
			input:    `{"a": 1}`,
			wantCode: "L105",
			wantMsg:  "missing, misplaced or repeated version declaration",
			wantHint: `start the top-level object with "$lkal": "0.1"`,
		},
		{
			name:     "L005_details",
			input:    `{"$lkal": "0.1", "a": 9223372036854775808}`,
			wantCode: "L005",
			wantMsg:  "number out of range",
			wantHint: "keep integers within int64 and numbers within 100 digits",
		},
		{
			name:     "L201_details",
			input:    `{"$lkal": "0.1", "a": 1, "a": 2}`,
			wantCode: "L201",
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L204_details",
			input:    `{"$lkal": "0.1", "a": [1, "two"]}`,
			wantCode: "L204",
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L205_details",
			input:    `{"$lkal": "0.1", "$foo": 1}`,
			wantCode: "L205",
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L207_details",
			input:    `{"$lkal": "0.2"}`,
			wantCode: "L207",
			wantMsg:  "unsupported version",
			wantHint: `this implementation accepts only "0.1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := FromJSON([]byte(tt.input))
			if err == nil {
				t.Fatalf("expected error from FromJSON, got nil error, file: %v", f)
			}
			if f != nil {
				t.Errorf("expected nil File on error, got: %v", f)
			}

			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}
			if errList == nil || len(errList.Errors) == 0 {
				t.Fatalf("expected non-empty ErrorList")
			}

			first := errList.Errors[0]
			if first == nil {
				t.Fatalf("expected non-nil first error")
			}
			if first.File != "" {
				t.Errorf("File mismatch: got %q, want %q", first.File, "")
			}
			if first.Code != tt.wantCode {
				t.Errorf("Code mismatch: got %q, want %q", first.Code, tt.wantCode)
			}
			if first.Msg != tt.wantMsg {
				t.Errorf("Msg mismatch: got %q, want %q", first.Msg, tt.wantMsg)
			}
			if first.Hint != tt.wantHint {
				t.Errorf("Hint mismatch: got %q, want %q", first.Hint, tt.wantHint)
			}
		})
	}

	t.Run("render_exact_example", func(t *testing.T) {
		f, err := FromJSON([]byte(`{"$lkal": "0.1", "a": }`))
		if err == nil {
			t.Fatalf("expected error, got nil file: %v", f)
		}
		want := "1:23: L301 invalid JSON\n  hint: fix the JSON syntax"
		if got := err.Error(); got != want {
			t.Errorf("Error() rendering mismatch:\ngot:  %q\nwant: %q", got, want)
		}
	})
}

func TestJSONTypedNilErrorList(t *testing.T) {
	var el *ErrorList
	if res := jsonTExtractErrors(el); res != nil {
		t.Errorf("expected nil result for typed-nil ErrorList, got %v", res)
	}
}
