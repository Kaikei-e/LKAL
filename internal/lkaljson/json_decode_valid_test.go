package lkaljson_test

import . "github.com/Kaikei-e/LKAL"

import (
	"strings"
	"testing"
)

func TestFromJSONValid(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantVersion string
		wantRecord  Record
	}{
		{
			name:        "scalars_all_types",
			input:       `{"$lkal": "0.1", "s": "hello", "i": 42, "d": 0.25, "b": true, "proto": {"$enum": "tcp"}, "mem": {"$qty": "512Mi"}, "ref": {"$ref": "secret/db-creds"}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "s", Value: String("hello")},
					{Key: "i", Value: Int(42)},
					{Key: "d", Value: Decimal("0.25")},
					{Key: "b", Value: Bool(true)},
					{Key: "proto", Value: Enum("tcp")},
					{Key: "mem", Value: Quantity{Number: "512", Unit: "Mi"}},
					{Key: "ref", Value: Ref{"secret", "db-creds"}},
				},
			},
		},
		{
			name:        "number_normalizations",
			input:       `{"$lkal": "0.1", "d1": 1.50, "d2": 2.00, "i1": -0, "d3": -0.0, "max_int": 9223372036854775807}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "d1", Value: Decimal("1.5")},
					{Key: "d2", Value: Decimal("2.0")},
					{Key: "i1", Value: Int(0)},
					{Key: "d3", Value: Decimal("0.0")},
					{Key: "max_int", Value: Int(9223372036854775807)},
				},
			},
		},
		{
			name:        "quantity_normalizations",
			input:       `{"$lkal": "0.1", "q1": {"$qty": "1.50Gi"}, "q2": {"$qty": "-0Mi"}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "q1", Value: Quantity{Number: "1.5", Unit: "Gi"}},
					{Key: "q2", Value: Quantity{Number: "0", Unit: "Mi"}},
				},
			},
		},
		{
			name:        "ref_with_quoted_segments",
			input:       `{"$lkal": "0.1", "r": {"$ref": "a/\"b c\""}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "r", Value: Ref{"a", "b c"}},
				},
			},
		},
		{
			name:        "ref_decoded_escape",
			input:       `{"$lkal": "0.1", "r": {"$ref": "\"\\u{41}\""}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "r", Value: Ref{"A"}},
				},
			},
		},
		{
			name:        "decimal_100_digits_accepted",
			input:       `{"$lkal": "0.1", "d": ` + strings.Repeat("1", 50) + "." + strings.Repeat("2", 50) + `}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "d", Value: Decimal(strings.Repeat("1", 50) + "." + strings.Repeat("2", 50))},
				},
			},
		},
		{
			name:        "collection_with_dollar_label",
			input:       `{"$lkal": "0.1", "c": {"$coll": {"$special": {"v": 1}}}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{
						Key: "c",
						Value: Collection{
							Elems: []Elem{
								{
									Label: "$special",
									Body: Record{
										Fields: []Field{
											{Key: "v", Value: Int(1)},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "list_of_enums",
			input:       `{"$lkal": "0.1", "enums": [{"$enum": "tcp"}, {"$enum": "udp"}]}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "enums", Value: List{Enum("tcp"), Enum("udp")}},
				},
			},
		},
		{
			name:        "list_of_quantities_different_units",
			input:       `{"$lkal": "0.1", "qtys": [{"$qty": "10m"}, {"$qty": "20s"}]}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "qtys", Value: List{Quantity{Number: "10", Unit: "m"}, Quantity{Number: "20", Unit: "s"}}},
				},
			},
		},
		{
			name:        "empty_list",
			input:       `{"$lkal": "0.1", "items": []}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "items", Value: List{}},
				},
			},
		},
		{
			name:        "empty_record",
			input:       `{"$lkal": "0.1", "rec": {}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "rec", Value: Record{}},
				},
			},
		},
		{
			name:        "nested_collections",
			input:       `{"$lkal": "0.1", "outer": {"$coll": {"first": {"inner": {"$coll": {"nested": {"v": 10}}}}}}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{
						Key: "outer",
						Value: Collection{
							Elems: []Elem{
								{
									Label: "first",
									Body: Record{
										Fields: []Field{
											{
												Key: "inner",
												Value: Collection{
													Elems: []Elem{
														{
															Label: "nested",
															Body: Record{
																Fields: []Field{
																	{Key: "v", Value: Int(10)},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "crlf_and_tab_whitespace",
			input:       "{\r\n\t\"$lkal\":\t\"0.1\",\r\n\t\"a\":\t1\r\n}\r\n",
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "a", Value: Int(1)},
				},
			},
		},
		{
			name:        "emoji_surrogate_escape",
			input:       `{"$lkal": "0.1", "e": "\uD83D\uDE00"}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "e", Value: String("😀")},
				},
			},
		},
		{
			name:        "members_after_record_key_order_preserved",
			input:       `{"$lkal": "0.1", "rec": {"b": 2, "a": 1}, "after": 3}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{
						Key: "rec",
						Value: Record{
							Fields: []Field{
								{Key: "b", Value: Int(2)},
								{Key: "a", Value: Int(1)},
							},
						},
					},
					{Key: "after", Value: Int(3)},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := FromJSON([]byte(tt.input))
			if err != nil {
				t.Fatalf("FromJSON returned unexpected error: %v", err)
			}
			if f == nil {
				t.Fatalf("FromJSON returned nil File")
			}

			if gotVer := f.Version(); gotVer != tt.wantVersion {
				t.Errorf("Version mismatch: got %q, want %q", gotVer, tt.wantVersion)
			}

			gotData := f.Data()
			if !Equal(gotData, tt.wantRecord) {
				t.Errorf("Equal(Data(), wantRecord) = false:\ngot:  %#v\nwant: %#v", gotData, tt.wantRecord)
			}

			if err := jsonTCheckOrder(gotData, tt.wantRecord); err != nil {
				t.Errorf("order check failed: %v", err)
			}
		})
	}
}
