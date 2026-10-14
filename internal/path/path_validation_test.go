package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"strings"
	"testing"
)

func TestSetNumberChecks(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		tests := []struct {
			name       string
			src        string
			path       string
			value      Value
			wantFormat string
			wantGet    Value
		}{
			{
				name:       "quantity_large_integer_no_int64_limit",
				src:        "@lkal \"0.1\"\n",
				path:       "q",
				value:      Quantity{Number: "99999999999999999999", Unit: "Mi"},
				wantFormat: "@lkal \"0.1\"\n\nq = 99999999999999999999Mi\n",
				wantGet:    Quantity{Number: "99999999999999999999", Unit: "Mi"},
			},
			{
				name:       "quantity_negative_overflow_int64",
				src:        "@lkal \"0.1\"\n",
				path:       "q",
				value:      Quantity{Number: "-9223372036854775809", Unit: "Gi"},
				wantFormat: "@lkal \"0.1\"\n\nq = -9223372036854775809Gi\n",
				wantGet:    Quantity{Number: "-9223372036854775809", Unit: "Gi"},
			},
			{
				name: "list_of_quantities_overflow_int64",
				src:  "@lkal \"0.1\"\n",
				path: "q",
				value: List{
					Quantity{Number: "-9223372036854775809", Unit: "Gi"},
					Quantity{Number: "-9223372036854775810", Unit: "Gi"},
				},
				wantFormat: "@lkal \"0.1\"\n\nq = [-9223372036854775809Gi, -9223372036854775810Gi]\n",
				wantGet: List{
					Quantity{Number: "-9223372036854775809", Unit: "Gi"},
					Quantity{Number: "-9223372036854775810", Unit: "Gi"},
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "test.lkal", tc.src)
				err := f.Set(tc.path, tc.value)
				if err != nil {
					t.Fatalf("Set(%q, %#v) unexpected error: %v", tc.path, tc.value, err)
				}
				fmtBytes := f.Format()
				if fmtBytes == nil {
					t.Fatalf("Format() returned nil, want non-nil")
				}
				gotFmt := string(fmtBytes)
				if gotFmt != tc.wantFormat {
					t.Errorf("Format() mismatch:\ngot:\n%s\nwant:\n%s", gotFmt, tc.wantFormat)
				}
				parsed := pathTParse(t, "", gotFmt)
				pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())

				gotVal, getErr := f.Get(tc.path)
				if getErr != nil {
					t.Fatalf("Get(%q) unexpected error: %v", tc.path, getErr)
				}
				if gotVal == nil {
					t.Fatalf("Get(%q) returned nil Value", tc.path)
				}
				pathTCheckEqualAndOrder(t, gotVal, tc.wantGet)
			})
		}

		t.Run("quantity_parsed_from_text_then_set_get_roundtrip", func(t *testing.T) {
			const parsedSrc = "@lkal \"0.1\"\n\nq = 99999999999999999999Mi\n"
			f := pathTParse(t, "test.lkal", parsedSrc)
			origFmtBytes := f.Format()
			if origFmtBytes == nil {
				t.Fatalf("Format() returned nil, want non-nil")
			}
			origFmt := string(origFmtBytes)

			gotVal, getErr := f.Get("q")
			if getErr != nil {
				t.Fatalf("Get(%q) unexpected error: %v", "q", getErr)
			}
			if gotVal == nil {
				t.Fatalf("Get(%q) returned nil Value", "q")
			}
			wantVal := Quantity{Number: "99999999999999999999", Unit: "Mi"}
			pathTCheckEqualAndOrder(t, gotVal, wantVal)

			err := f.Set("q", gotVal)
			if err != nil {
				t.Fatalf("Set(%q, Get(%q)) unexpected error: %v", "q", "q", err)
			}
			newFmtBytes := f.Format()
			if newFmtBytes == nil {
				t.Fatalf("Format() after Set returned nil, want non-nil")
			}
			newFmt := string(newFmtBytes)
			if newFmt != origFmt {
				t.Errorf("Format() mismatch:\ngot:\n%s\nwant:\n%s", newFmt, origFmt)
			}
			parsed := pathTParse(t, "", newFmt)
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())
		})
	})

	t.Run("negative_hints", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			value    Value
			wantCode string
			wantMsg  string
			wantHint string
		}{
			{
				name:     "decimal_101_digits_without_point",
				path:     "dec",
				value:    Decimal(strings.Repeat("1", 101)),
				wantCode: "L403",
				wantMsg:  "invalid value",
				wantHint: "write a decimal such as 0.25",
			},
			{
				name:     "quantity_101_digits_with_invalid_char",
				path:     "qty",
				value:    Quantity{Number: strings.Repeat("1", 101) + "x", Unit: "Mi"},
				wantCode: "L403",
				wantMsg:  "invalid value",
				wantHint: "write a quantity such as 512Mi",
			},
			{
				name:     "decimal_101_digits_with_point_range_exceeded",
				path:     "dec",
				value:    Decimal("1." + strings.Repeat("1", 100)),
				wantCode: "L403",
				wantMsg:  "invalid value",
				wantHint: "keep numbers within 100 digits",
			},
			{
				name:     "quantity_unit_with_digit",
				path:     "qty",
				value:    Quantity{Number: "1.5", Unit: "M1"},
				wantCode: "L403",
				wantMsg:  "invalid value",
				wantHint: "write a quantity such as 512Mi",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "test.lkal", "@lkal \"0.1\"\n")
				origFmtBytes := f.Format()
				if origFmtBytes == nil {
					t.Fatalf("Format() returned nil, want non-nil")
				}
				origFmt := string(origFmtBytes)

				err := f.Set(tc.path, tc.value)
				pathTCheckSingleError(t, err, "test.lkal", tc.wantCode, tc.wantMsg, tc.wantHint)

				postFmtBytes := f.Format()
				if postFmtBytes == nil {
					t.Fatalf("Format() returned nil after failed Set, want non-nil")
				}
				postFmt := string(postFmtBytes)
				if postFmt != origFmt {
					t.Errorf("File changed after failed Set:\ngot:\n%s\nwant:\n%s", postFmt, origFmt)
				}
			})
		}
	})
}

func TestSetL403Order(t *testing.T) {
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
			name:     "record_dollar_key_precedes_field_value_validation",
			src:      "@lkal \"0.1\"\n",
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "a", Value: Decimal("1.")}, {Key: "$b", Value: Int(1)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "rename keys that start with $",
		},
		{
			name:     "record_duplicate_key_precedes_field_value_validation",
			src:      "@lkal \"0.1\"\n",
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "a", Value: Decimal("1.")}, {Key: "a", Value: Int(1)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "remove the duplicate key",
		},
		{
			name: "collection_duplicate_label_precedes_body_validation",
			src:  "@lkal \"0.1\"\n",
			path: "coll",
			value: Collection{Elems: []Elem{
				{Label: "x", Body: Record{Fields: []Field{{Key: "a", Value: Decimal("1.")}}}},
				{Label: "x", Body: Record{}},
			}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "remove the duplicate label",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", tc.src)
			origFmtBytes := f.Format()
			if origFmtBytes == nil {
				t.Fatalf("Format() returned nil, want non-nil")
			}
			origFmt := string(origFmtBytes)

			err := f.Set(tc.path, tc.value)
			pathTCheckSingleError(t, err, "test.lkal", tc.wantCode, tc.wantMsg, tc.wantHint)

			postFmtBytes := f.Format()
			if postFmtBytes == nil {
				t.Fatalf("Format() returned nil after failed Set, want non-nil")
			}
			postFmt := string(postFmtBytes)
			if postFmt != origFmt {
				t.Errorf("File changed after failed Set:\ngot:\n%s\nwant:\n%s", postFmt, origFmt)
			}
		})
	}
}
