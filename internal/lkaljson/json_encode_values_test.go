package lkaljson_test

import . "github.com/Kaikei-e/LKAL"

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToJSONValues(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantJSON string
	}{
		{
			name:     "string",
			src:      "@lkal \"0.1\"\nstr = \"hello\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"str\": \"hello\"\n}\n",
		},
		{
			name:     "int_positive",
			src:      "@lkal \"0.1\"\nnum = 42\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"num\": 42\n}\n",
		},
		{
			name:     "int_negative",
			src:      "@lkal \"0.1\"\nnum = -42\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"num\": -42\n}\n",
		},
		{
			name:     "decimal_2_0",
			src:      "@lkal \"0.1\"\ndec = 2.0\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"dec\": 2.0\n}\n",
		},
		{
			name:     "decimal_neg_1_5",
			src:      "@lkal \"0.1\"\ndec = -1.5\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"dec\": -1.5\n}\n",
		},
		{
			name:     "decimal_0_25",
			src:      "@lkal \"0.1\"\ndec = 0.25\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"dec\": 0.25\n}\n",
		},
		{
			name:     "bool",
			src:      "@lkal \"0.1\"\nb = true\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"b\": true\n}\n",
		},
		{
			name:     "enum",
			src:      "@lkal \"0.1\"\nproto = .tcp\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"proto\": {\n    \"$enum\": \"tcp\"\n  }\n}\n",
		},
		{
			name:     "quantity_512mi",
			src:      "@lkal \"0.1\"\nmem = 512Mi\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"mem\": {\n    \"$qty\": \"512Mi\"\n  }\n}\n",
		},
		{
			name:     "quantity_1_5gi",
			src:      "@lkal \"0.1\"\nmem = 1.5Gi\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"mem\": {\n    \"$qty\": \"1.5Gi\"\n  }\n}\n",
		},
		{
			name:     "ref_bare_segments",
			src:      "@lkal \"0.1\"\ntarget = &secret/db-creds\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"secret/db-creds\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_segment_with_space",
			src:      "@lkal \"0.1\"\ntarget = &secret/\"db creds\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"secret/\\\"db creds\\\"\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_simple_segment_canonical_bare",
			src:      "@lkal \"0.1\"\ntarget = &a/\"b\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"a/b\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_empty_segment",
			src:      "@lkal \"0.1\"\ntarget = &a/\"\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"a/\\\"\\\"\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_slash_segment",
			src:      "@lkal \"0.1\"\ntarget = &labels/\"app.kubernetes.io/name\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"labels/\\\"app.kubernetes.io/name\\\"\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_segment_holding_u0001",
			src:      "@lkal \"0.1\"\ntarget = &a/\"\\u{1}\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"a/\\\"\\\\u{1}\\\"\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_segment_holding_raw_tab",
			src:      "@lkal \"0.1\"\ntarget = &a/\"\t\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"a/\\\"\\t\\\"\"\n  }\n}\n",
		},
		{
			name:     "list",
			src:      "@lkal \"0.1\"\nitems = [1, 2]\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"items\": [\n    1,\n    2\n  ]\n}\n",
		},
		{
			name:     "record",
			src:      "@lkal \"0.1\"\nsub {\n  leaf = \"ok\"\n}\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"sub\": {\n    \"leaf\": \"ok\"\n  }\n}\n",
		},
		{
			name:     "collection",
			src:      "@lkal \"0.1\"\nport \"http\" {\n  number = 80\n}\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"port\": {\n    \"$coll\": {\n      \"http\": {\n        \"number\": 80\n      }\n    }\n  }\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.name+".lkal", []byte(tt.src))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if f == nil {
				t.Fatalf("Parse returned nil File")
			}

			got, err := ToJSON(f, false)
			if err != nil {
				t.Fatalf("ToJSON failed: %v", err)
			}

			diffs := jsonTDiffJSON(got, []byte(tt.wantJSON))
			if len(diffs) > 0 {
				t.Errorf("token differences found:\n%s\ngot:\n%s\nwant:\n%s",
					strings.Join(diffs, "\n"), string(got), tt.wantJSON)
			}
			if string(got) != tt.wantJSON {
				t.Errorf("exact bytes mismatch:\ngot:\n%s\nwant:\n%s", string(got), tt.wantJSON)
			}
		})
	}
}

func TestToJSONDeepDocuments(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "record_1000_levels",
			src:  "@lkal \"0.1\"\n" + strings.Repeat("a {\n", 1000) + "x = 1\n" + strings.Repeat("}\n", 1000),
		},
		{
			name: "element_1000_levels",
			src:  "@lkal \"0.1\"\n" + strings.Repeat("c \"l\" {\n", 1000) + "x = 1\n" + strings.Repeat("}\n", 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.name+".lkal", []byte(tt.src))
			if err != nil {
				t.Fatalf("Parse(%s) failed: got %v, want nil", tt.name, err)
			}
			if f == nil {
				t.Fatalf("Parse(%s) returned nil File: got nil, want non-nil", tt.name)
			}

			jLKAL, err := ToJSON(f, false)
			if err != nil {
				t.Fatalf("ToJSON(f, false) failed: got %v, want nil", err)
			}
			if jLKAL == nil {
				t.Fatalf("ToJSON(f, false) returned nil bytes: got nil, want non-nil")
			}
			if !json.Valid(jLKAL) {
				t.Errorf("ToJSON(f, false) output is not valid JSON: got invalid, want valid")
			}

			jPlain, err := ToJSON(f, true)
			if err != nil {
				t.Fatalf("ToJSON(f, true) failed: got %v, want nil", err)
			}
			if jPlain == nil {
				t.Fatalf("ToJSON(f, true) returned nil bytes: got nil, want non-nil")
			}
			if !json.Valid(jPlain) {
				t.Errorf("ToJSON(f, true) output is not valid JSON: got invalid, want valid")
			}

			g, err := FromJSON(jLKAL)
			if err != nil {
				t.Fatalf("FromJSON(ToJSON(f, false)) failed: got %v, want nil", err)
			}
			if g == nil {
				t.Fatalf("FromJSON(ToJSON(f, false)) returned nil File: got nil, want non-nil")
			}

			if !Equal(g.Data(), f.Data()) {
				t.Errorf("FromJSON(ToJSON(f, false)) Data() mismatch:\ngot:  %#v\nwant: %#v", g.Data(), f.Data())
			}
		})
	}
}
