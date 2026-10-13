package lkal

import (
	"math"
	"testing"
)

type valueTestCase struct {
	name string
	a    Value
	b    Value
	want bool
}

type valueTypedSample struct {
	name string
	val  Value
}

func valueRunTests(t *testing.T, tests []valueTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Equal(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Equal(%#v, %#v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func valueBuildRecordAllTypes() Record {
	return Record{
		Fields: []Field{
			{Key: "str", Value: String("val")},
			{Key: "int", Value: Int(42)},
			{Key: "dec", Value: Decimal("3.14")},
			{Key: "bool", Value: Bool(true)},
			{Key: "enum", Value: Enum("ready")},
			{Key: "qty", Value: Quantity{Number: "100", Unit: "ms"}},
			{Key: "ref", Value: Ref{"sys", "clock"}},
			{Key: "list", Value: List{Int(1), Int(2)}},
			{Key: "rec", Value: Record{Fields: []Field{{Key: "inner", Value: String("ok")}}}},
			{Key: "coll", Value: Collection{Elems: []Elem{{Label: "c1", Body: Record{Fields: []Field{{Key: "flag", Value: Bool(true)}}}}}}},
		},
	}
}

func TestEqual_Nil(t *testing.T) {
	tests := []valueTestCase{
		{name: "nil_and_nil", a: nil, b: nil, want: true},
		{name: "nil_and_string", a: nil, b: String(""), want: false},
		{name: "string_and_nil", a: String(""), b: nil, want: false},
		{name: "nil_and_int", a: nil, b: Int(0), want: false},
		{name: "int_and_nil", a: Int(0), b: nil, want: false},
		{name: "nil_and_decimal", a: nil, b: Decimal("0.0"), want: false},
		{name: "decimal_and_nil", a: Decimal("0.0"), b: nil, want: false},
		{name: "nil_and_bool", a: nil, b: Bool(false), want: false},
		{name: "bool_and_nil", a: Bool(false), b: nil, want: false},
		{name: "nil_and_enum", a: nil, b: Enum("tcp"), want: false},
		{name: "enum_and_nil", a: Enum("tcp"), b: nil, want: false},
		{name: "nil_and_quantity", a: nil, b: Quantity{Number: "1", Unit: "s"}, want: false},
		{name: "quantity_and_nil", a: Quantity{Number: "1", Unit: "s"}, b: nil, want: false},
		{name: "nil_and_ref", a: nil, b: Ref{"a"}, want: false},
		{name: "ref_and_nil", a: Ref{"a"}, b: nil, want: false},
		{name: "nil_and_list", a: nil, b: List{Int(1)}, want: false},
		{name: "list_and_nil", a: List{Int(1)}, b: nil, want: false},
		{name: "nil_and_record", a: nil, b: Record{}, want: false},
		{name: "record_and_nil", a: Record{}, b: nil, want: false},
		{name: "nil_and_collection", a: nil, b: Collection{}, want: false},
		{name: "collection_and_nil", a: Collection{}, b: nil, want: false},
		{name: "nil_and_nil_ref", a: nil, b: Ref(nil), want: false},
		{name: "nil_ref_and_nil", a: Ref(nil), b: nil, want: false},
		{name: "nil_and_nil_list", a: nil, b: List(nil), want: false},
		{name: "nil_list_and_nil", a: List(nil), b: nil, want: false},
		{name: "nil_and_nil_record_fields", a: nil, b: Record{Fields: nil}, want: false},
		{name: "nil_record_fields_and_nil", a: Record{Fields: nil}, b: nil, want: false},
		{name: "nil_and_nil_collection_elems", a: nil, b: Collection{Elems: nil}, want: false},
		{name: "nil_collection_elems_and_nil", a: Collection{Elems: nil}, b: nil, want: false},
	}
	valueRunTests(t, tests)
}

func TestEqual_TypeMismatch(t *testing.T) {
	explicitTests := []valueTestCase{
		{name: "int_vs_decimal", a: Int(1), b: Decimal("1.0"), want: false},
		{name: "decimal_vs_int", a: Decimal("1.0"), b: Int(1), want: false},
		{name: "int_vs_decimal_same_text", a: Int(1), b: Decimal("1"), want: false},
		{name: "string_vs_enum", a: String("a"), b: Enum("a"), want: false},
		{name: "enum_vs_string", a: Enum("a"), b: String("a"), want: false},
		{name: "record_vs_collection", a: Record{}, b: Collection{}, want: false},
		{name: "collection_vs_record", a: Collection{}, b: Record{}, want: false},
		{name: "string_vs_int", a: String("1"), b: Int(1), want: false},
		{name: "string_vs_bool", a: String("true"), b: Bool(true), want: false},
		{name: "string_vs_quantity", a: String("512Mi"), b: Quantity{Number: "512", Unit: "Mi"}, want: false},
		{name: "string_vs_ref", a: String("secret/db"), b: Ref{"secret", "db"}, want: false},
		{name: "ref_vs_list", a: Ref{"a"}, b: List{String("a")}, want: false},
		{name: "list_vs_ref", a: List{String("a")}, b: Ref{"a"}, want: false},
		{name: "list_vs_record", a: List{}, b: Record{}, want: false},
		{name: "list_vs_collection", a: List{}, b: Collection{}, want: false},
		{name: "quantity_vs_int", a: Quantity{Number: "1", Unit: "m"}, b: Int(1), want: false},
		{name: "quantity_vs_decimal", a: Quantity{Number: "1.0", Unit: "m"}, b: Decimal("1.0"), want: false},
		{name: "bool_vs_int", a: Bool(true), b: Int(1), want: false},
		{name: "enum_vs_ref", a: Enum("a"), b: Ref{"a"}, want: false},
	}
	valueRunTests(t, explicitTests)

	samples := []valueTypedSample{
		{name: "string", val: String("sample")},
		{name: "int", val: Int(42)},
		{name: "decimal", val: Decimal("42.0")},
		{name: "bool", val: Bool(true)},
		{name: "enum", val: Enum("sample")},
		{name: "quantity", val: Quantity{Number: "42", Unit: "m"}},
		{name: "ref", val: Ref{"sample"}},
		{name: "list", val: List{Int(42)}},
		{name: "record", val: Record{Fields: []Field{{Key: "k", Value: Int(42)}}}},
		{name: "collection", val: Collection{Elems: []Elem{{Label: "k", Body: Record{Fields: []Field{{Key: "k", Value: Int(42)}}}}}}},
	}

	for i := 0; i < len(samples); i++ {
		for j := i + 1; j < len(samples); j++ {
			s1 := samples[i]
			s2 := samples[j]
			t.Run(s1.name+"_vs_"+s2.name, func(t *testing.T) {
				got := Equal(s1.val, s2.val)
				if got {
					t.Errorf("Equal(%#v, %#v) = true, want false", s1.val, s2.val)
				}
			})
			t.Run(s2.name+"_vs_"+s1.name, func(t *testing.T) {
				got := Equal(s2.val, s1.val)
				if got {
					t.Errorf("Equal(%#v, %#v) = true, want false", s2.val, s1.val)
				}
			})
		}
	}
}

func TestEqual_Scalars(t *testing.T) {
	tests := []valueTestCase{
		{name: "string_empty", a: String(""), b: String(""), want: true},
		{name: "string_identical", a: String("alpha"), b: String("alpha"), want: true},
		{name: "string_unicode", a: String("日本語テキスト"), b: String("日本語テキスト"), want: true},
		{name: "string_multiline", a: String("line1\nline2"), b: String("line1\nline2"), want: true},
		{name: "string_with_spaces", a: String("  indented  "), b: String("  indented  "), want: true},
		{name: "string_different", a: String("alpha"), b: String("beta"), want: false},
		{name: "string_case_sensitive", a: String("alpha"), b: String("Alpha"), want: false},
		{name: "string_prefix", a: String("alpha"), b: String("alph"), want: false},
		{name: "string_empty_vs_nonempty", a: String(""), b: String(" "), want: false},
		{name: "string_newline_diff", a: String("a\nb"), b: String("a\r\nb"), want: false},

		{name: "int_zero", a: Int(0), b: Int(0), want: true},
		{name: "int_positive", a: Int(123456789), b: Int(123456789), want: true},
		{name: "int_negative", a: Int(-987654321), b: Int(-987654321), want: true},
		{name: "int_max", a: Int(math.MaxInt64), b: Int(math.MaxInt64), want: true},
		{name: "int_min", a: Int(math.MinInt64), b: Int(math.MinInt64), want: true},
		{name: "int_diff_small", a: Int(0), b: Int(1), want: false},
		{name: "int_diff_sign", a: Int(1), b: Int(-1), want: false},
		{name: "int_diff_large", a: Int(math.MaxInt64), b: Int(math.MinInt64), want: false},
		{name: "int_off_by_one", a: Int(100), b: Int(101), want: false},

		{name: "decimal_identical", a: Decimal("0.25"), b: Decimal("0.25"), want: true},
		{name: "decimal_negative", a: Decimal("-1.5"), b: Decimal("-1.5"), want: true},
		{name: "decimal_zero", a: Decimal("0.0"), b: Decimal("0.0"), want: true},
		{name: "decimal_large", a: Decimal("12345678901234567890.123456789"), b: Decimal("12345678901234567890.123456789"), want: true},
		{name: "decimal_trailing_zeros_diff", a: Decimal("0.25"), b: Decimal("0.250"), want: false},
		{name: "decimal_precision_diff", a: Decimal("1.0"), b: Decimal("1.00"), want: false},
		{name: "decimal_integer_notation_diff", a: Decimal("1"), b: Decimal("1.0"), want: false},
		{name: "decimal_zero_notation_diff", a: Decimal("0"), b: Decimal("0.0"), want: false},
		{name: "decimal_sign_diff", a: Decimal("-0.5"), b: Decimal("0.5"), want: false},
		{name: "decimal_negative_zero", a: Decimal("-0.0"), b: Decimal("0.0"), want: false},
		{name: "decimal_diff_values", a: Decimal("0.25"), b: Decimal("0.75"), want: false},

		{name: "bool_true_true", a: Bool(true), b: Bool(true), want: true},
		{name: "bool_false_false", a: Bool(false), b: Bool(false), want: true},
		{name: "bool_true_false", a: Bool(true), b: Bool(false), want: false},
		{name: "bool_false_true", a: Bool(false), b: Bool(true), want: false},

		{name: "enum_identical", a: Enum("tcp"), b: Enum("tcp"), want: true},
		{name: "enum_another", a: Enum("running"), b: Enum("running"), want: true},
		{name: "enum_different", a: Enum("tcp"), b: Enum("udp"), want: false},
		{name: "enum_case_sensitive", a: Enum("tcp"), b: Enum("TCP"), want: false},
		{name: "enum_empty", a: Enum(""), b: Enum(""), want: true},
		{name: "enum_empty_vs_nonempty", a: Enum(""), b: Enum("val"), want: false},
		{name: "enum_prefix", a: Enum("sub"), b: Enum("subpath"), want: false},
	}
	valueRunTests(t, tests)
}

func TestEqual_Quantity(t *testing.T) {
	tests := []valueTestCase{
		{name: "quantity_identical_int_number", a: Quantity{Number: "512", Unit: "Mi"}, b: Quantity{Number: "512", Unit: "Mi"}, want: true},
		{name: "quantity_identical_dec_number", a: Quantity{Number: "0.25", Unit: "s"}, b: Quantity{Number: "0.25", Unit: "s"}, want: true},
		{name: "quantity_identical_negative", a: Quantity{Number: "-10", Unit: "deg"}, b: Quantity{Number: "-10", Unit: "deg"}, want: true},
		{name: "quantity_case_sensitive_unit_mi", a: Quantity{Number: "512", Unit: "Mi"}, b: Quantity{Number: "512", Unit: "mi"}, want: false},
		{name: "quantity_case_sensitive_unit_m", a: Quantity{Number: "250", Unit: "M"}, b: Quantity{Number: "250", Unit: "m"}, want: false},
		{name: "quantity_case_sensitive_unit_kb", a: Quantity{Number: "10", Unit: "KB"}, b: Quantity{Number: "10", Unit: "kb"}, want: false},
		{name: "quantity_case_sensitive_unit_gi", a: Quantity{Number: "1", Unit: "Gi"}, b: Quantity{Number: "1", Unit: "GI"}, want: false},
		{name: "quantity_unit_diff", a: Quantity{Number: "512", Unit: "Mi"}, b: Quantity{Number: "512", Unit: "Gi"}, want: false},
		{name: "quantity_unit_diff_time", a: Quantity{Number: "30", Unit: "s"}, b: Quantity{Number: "30", Unit: "m"}, want: false},
		{name: "quantity_number_diff", a: Quantity{Number: "512", Unit: "Mi"}, b: Quantity{Number: "256", Unit: "Mi"}, want: false},
		{name: "quantity_number_trailing_zeros", a: Quantity{Number: "1.0", Unit: "s"}, b: Quantity{Number: "1.00", Unit: "s"}, want: false},
		{name: "quantity_number_int_vs_dec", a: Quantity{Number: "1", Unit: "s"}, b: Quantity{Number: "1.0", Unit: "s"}, want: false},
		{name: "quantity_both_diff", a: Quantity{Number: "1", Unit: "s"}, b: Quantity{Number: "2", Unit: "m"}, want: false},
		{name: "quantity_empty_fields", a: Quantity{}, b: Quantity{}, want: true},
		{name: "quantity_empty_vs_number", a: Quantity{}, b: Quantity{Number: "1", Unit: ""}, want: false},
		{name: "quantity_empty_vs_unit", a: Quantity{}, b: Quantity{Number: "", Unit: "s"}, want: false},
	}
	valueRunTests(t, tests)
}

func TestEqual_Ref(t *testing.T) {
	tests := []valueTestCase{
		{name: "ref_identical_single", a: Ref{"web"}, b: Ref{"web"}, want: true},
		{name: "ref_identical_multi", a: Ref{"secret", "db-creds"}, b: Ref{"secret", "db-creds"}, want: true},
		{name: "ref_identical_three", a: Ref{"a", "b", "c"}, b: Ref{"a", "b", "c"}, want: true},
		{name: "ref_order_sensitive_two", a: Ref{"a", "b"}, b: Ref{"b", "a"}, want: false},
		{name: "ref_order_sensitive_three", a: Ref{"a", "b", "c"}, b: Ref{"a", "c", "b"}, want: false},
		{name: "ref_order_sensitive_reversed", a: Ref{"a", "b", "c"}, b: Ref{"c", "b", "a"}, want: false},
		{name: "ref_length_shorter", a: Ref{"a"}, b: Ref{"a", "b"}, want: false},
		{name: "ref_length_longer", a: Ref{"a", "b"}, b: Ref{"a"}, want: false},
		{name: "ref_segment_content_diff", a: Ref{"secret", "db1"}, b: Ref{"secret", "db2"}, want: false},
		{name: "ref_case_sensitive", a: Ref{"Secret", "db"}, b: Ref{"secret", "db"}, want: false},
		{name: "ref_empty_slice", a: Ref{}, b: Ref{}, want: true},
		{name: "ref_nil_slice", a: Ref(nil), b: Ref(nil), want: true},
		{name: "ref_empty_and_nil", a: Ref{}, b: Ref(nil), want: true},
		{name: "ref_nil_and_empty", a: Ref(nil), b: Ref{}, want: true},
		{name: "ref_empty_vs_nonempty", a: Ref{}, b: Ref{"a"}, want: false},
		{name: "ref_nil_vs_nonempty", a: Ref(nil), b: Ref{"a"}, want: false},
	}
	valueRunTests(t, tests)
}

func TestEqual_List(t *testing.T) {
	tests := []valueTestCase{
		{name: "list_empty_and_empty", a: List{}, b: List{}, want: true},
		{name: "list_nil_and_nil", a: List(nil), b: List(nil), want: true},
		{name: "list_empty_and_nil", a: List{}, b: List(nil), want: true},
		{name: "list_nil_and_empty", a: List(nil), b: List{}, want: true},
		{name: "list_identical_ints", a: List{Int(1), Int(2), Int(3)}, b: List{Int(1), Int(2), Int(3)}, want: true},
		{name: "list_identical_strings", a: List{String("a"), String("b")}, b: List{String("a"), String("b")}, want: true},
		{name: "list_identical_decimals", a: List{Decimal("1.5"), Decimal("2.5")}, b: List{Decimal("1.5"), Decimal("2.5")}, want: true},
		{name: "list_identical_bools", a: List{Bool(true), Bool(false)}, b: List{Bool(true), Bool(false)}, want: true},
		{name: "list_identical_enums", a: List{Enum("tcp"), Enum("udp")}, b: List{Enum("tcp"), Enum("udp")}, want: true},
		{
			name: "list_identical_quantities",
			a:    List{Quantity{Number: "1", Unit: "s"}, Quantity{Number: "2", Unit: "s"}},
			b:    List{Quantity{Number: "1", Unit: "s"}, Quantity{Number: "2", Unit: "s"}},
			want: true,
		},
		{name: "list_identical_refs", a: List{Ref{"a", "b"}, Ref{"c", "d"}}, b: List{Ref{"a", "b"}, Ref{"c", "d"}}, want: true},
		{name: "list_order_sensitive_ints", a: List{Int(1), Int(2)}, b: List{Int(2), Int(1)}, want: false},
		{name: "list_order_sensitive_strings", a: List{String("a"), String("b")}, b: List{String("b"), String("a")}, want: false},
		{name: "list_order_sensitive_reversed", a: List{Int(1), Int(2), Int(3)}, b: List{Int(3), Int(2), Int(1)}, want: false},
		{name: "list_length_diff_shorter", a: List{Int(1)}, b: List{Int(1), Int(2)}, want: false},
		{name: "list_length_diff_longer", a: List{Int(1), Int(2)}, b: List{Int(1)}, want: false},
		{name: "list_empty_vs_nonempty", a: List{}, b: List{Int(1)}, want: false},
		{name: "list_nil_vs_nonempty", a: List(nil), b: List{Int(1)}, want: false},
		{name: "list_element_value_diff", a: List{Int(1), Int(2)}, b: List{Int(1), Int(3)}, want: false},
		{name: "list_element_type_diff", a: List{Int(1)}, b: List{Decimal("1.0")}, want: false},
		{name: "list_with_nil_elements", a: List{nil}, b: List{nil}, want: true},
		{name: "list_with_nil_vs_nonnil_element", a: List{nil}, b: List{Int(1)}, want: false},
		{name: "list_with_nil_and_val", a: List{Int(1), nil}, b: List{Int(1), nil}, want: true},
		{name: "list_with_nil_and_val_swapped", a: List{Int(1), nil}, b: List{nil, Int(1)}, want: false},
	}
	valueRunTests(t, tests)
}

func TestEqual_Record(t *testing.T) {
	tests := []valueTestCase{
		{name: "record_empty_and_empty", a: Record{}, b: Record{}, want: true},
		{name: "record_nil_fields_and_nil_fields", a: Record{Fields: nil}, b: Record{Fields: nil}, want: true},
		{name: "record_empty_fields_and_nil_fields", a: Record{Fields: []Field{}}, b: Record{Fields: nil}, want: true},
		{name: "record_nil_fields_and_empty_fields", a: Record{Fields: nil}, b: Record{Fields: []Field{}}, want: true},
		{
			name: "record_single_field_identical",
			a:    Record{Fields: []Field{{Key: "k", Value: Int(1)}}},
			b:    Record{Fields: []Field{{Key: "k", Value: Int(1)}}},
			want: true,
		},
		{
			name: "record_two_fields_identical_order",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: String("two")}}},
			b:    Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: String("two")}}},
			want: true,
		},
		{
			name: "record_two_fields_scrambled_order",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: String("two")}}},
			b:    Record{Fields: []Field{{Key: "b", Value: String("two")}, {Key: "a", Value: Int(1)}}},
			want: true,
		},
		{
			name: "record_three_fields_scrambled_order",
			a:    Record{Fields: []Field{{Key: "x", Value: Int(10)}, {Key: "y", Value: Int(20)}, {Key: "z", Value: Int(30)}}},
			b:    Record{Fields: []Field{{Key: "z", Value: Int(30)}, {Key: "x", Value: Int(10)}, {Key: "y", Value: Int(20)}}},
			want: true,
		},
		{
			name: "record_four_fields_reverse_order",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(2)}, {Key: "c", Value: Int(3)}, {Key: "d", Value: Int(4)}}},
			b:    Record{Fields: []Field{{Key: "d", Value: Int(4)}, {Key: "c", Value: Int(3)}, {Key: "b", Value: Int(2)}, {Key: "a", Value: Int(1)}}},
			want: true,
		},
		{
			name: "record_diff_field_value",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			b:    Record{Fields: []Field{{Key: "a", Value: Int(2)}}},
			want: false,
		},
		{
			name: "record_diff_field_value_second",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(2)}}},
			b:    Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(3)}}},
			want: false,
		},
		{
			name: "record_diff_field_value_type",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			b:    Record{Fields: []Field{{Key: "a", Value: Decimal("1.0")}}},
			want: false,
		},
		{
			name: "record_diff_key_name",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			b:    Record{Fields: []Field{{Key: "b", Value: Int(1)}}},
			want: false,
		},
		{
			name: "record_key_case_sensitive",
			a:    Record{Fields: []Field{{Key: "key", Value: Int(1)}}},
			b:    Record{Fields: []Field{{Key: "Key", Value: Int(1)}}},
			want: false,
		},
		{
			name: "record_missing_key",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			b:    Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(2)}}},
			want: false,
		},
		{
			name: "record_extra_key",
			a:    Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(2)}}},
			b:    Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			want: false,
		},
		{
			name: "record_empty_vs_nonempty",
			a:    Record{},
			b:    Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			want: false,
		},
		{
			name: "record_nil_value_in_field",
			a:    Record{Fields: []Field{{Key: "a", Value: nil}}},
			b:    Record{Fields: []Field{{Key: "a", Value: nil}}},
			want: true,
		},
		{
			name: "record_nil_vs_nonnil_field_value",
			a:    Record{Fields: []Field{{Key: "a", Value: nil}}},
			b:    Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			want: false,
		},
		{
			name: "record_nil_field_value_reordered",
			a:    Record{Fields: []Field{{Key: "a", Value: nil}, {Key: "b", Value: Int(1)}}},
			b:    Record{Fields: []Field{{Key: "b", Value: Int(1)}, {Key: "a", Value: nil}}},
			want: true,
		},
	}
	valueRunTests(t, tests)
}

func TestEqual_Collection(t *testing.T) {
	recWeb := Record{Fields: []Field{{Key: "port", Value: Int(80)}}}
	recDB := Record{Fields: []Field{{Key: "port", Value: Int(5432)}}}
	recMetrics := Record{Fields: []Field{{Key: "port", Value: Int(9090)}}}

	tests := []valueTestCase{
		{name: "collection_empty_and_empty", a: Collection{}, b: Collection{}, want: true},
		{name: "collection_nil_elems_and_nil_elems", a: Collection{Elems: nil}, b: Collection{Elems: nil}, want: true},
		{name: "collection_empty_elems_and_nil_elems", a: Collection{Elems: []Elem{}}, b: Collection{Elems: nil}, want: true},
		{name: "collection_nil_elems_and_empty_elems", a: Collection{Elems: nil}, b: Collection{Elems: []Elem{}}, want: true},
		{
			name: "collection_single_elem_identical",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}}},
			b:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}}},
			want: true,
		},
		{
			name: "collection_two_elems_same_order",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}, {Label: "db", Body: recDB}}},
			b:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}, {Label: "db", Body: recDB}}},
			want: true,
		},
		{
			name: "collection_two_elems_scrambled_order",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}, {Label: "db", Body: recDB}}},
			b:    Collection{Elems: []Elem{{Label: "db", Body: recDB}, {Label: "web", Body: recWeb}}},
			want: true,
		},
		{
			name: "collection_three_elems_scrambled_order",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}, {Label: "db", Body: recDB}, {Label: "metrics", Body: recMetrics}}},
			b:    Collection{Elems: []Elem{{Label: "metrics", Body: recMetrics}, {Label: "web", Body: recWeb}, {Label: "db", Body: recDB}}},
			want: true,
		},
		{
			name: "collection_scrambled_elems_and_scrambled_body_fields",
			a: Collection{Elems: []Elem{
				{Label: "web", Body: Record{Fields: []Field{{Key: "p", Value: Int(80)}, {Key: "proto", Value: Enum("tcp")}}}},
				{Label: "db", Body: Record{Fields: []Field{{Key: "host", Value: String("localhost")}, {Key: "port", Value: Int(5432)}}}},
			}},
			b: Collection{Elems: []Elem{
				{Label: "db", Body: Record{Fields: []Field{{Key: "port", Value: Int(5432)}, {Key: "host", Value: String("localhost")}}}},
				{Label: "web", Body: Record{Fields: []Field{{Key: "proto", Value: Enum("tcp")}, {Key: "p", Value: Int(80)}}}},
			}},
			want: true,
		},
		{
			name: "collection_diff_body_value",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}}},
			b:    Collection{Elems: []Elem{{Label: "web", Body: Record{Fields: []Field{{Key: "port", Value: Int(8080)}}}}}},
			want: false,
		},
		{
			name: "collection_diff_label",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}}},
			b:    Collection{Elems: []Elem{{Label: "api", Body: recWeb}}},
			want: false,
		},
		{
			name: "collection_label_case_sensitive",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}}},
			b:    Collection{Elems: []Elem{{Label: "Web", Body: recWeb}}},
			want: false,
		},
		{
			name: "collection_missing_elem",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}}},
			b:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}, {Label: "db", Body: recDB}}},
			want: false,
		},
		{
			name: "collection_extra_elem",
			a:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}, {Label: "db", Body: recDB}}},
			b:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}}},
			want: false,
		},
		{
			name: "collection_empty_vs_nonempty",
			a:    Collection{},
			b:    Collection{Elems: []Elem{{Label: "web", Body: recWeb}}},
			want: false,
		},
	}
	valueRunTests(t, tests)
}

func TestEqual_Nested(t *testing.T) {
	tests := []valueTestCase{
		{
			name: "nested_record_identical",
			a:    Record{Fields: []Field{{Key: "child", Value: Record{Fields: []Field{{Key: "x", Value: Int(1)}}}}}},
			b:    Record{Fields: []Field{{Key: "child", Value: Record{Fields: []Field{{Key: "x", Value: Int(1)}}}}}},
			want: true,
		},
		{
			name: "nested_record_inner_fields_reordered",
			a:    Record{Fields: []Field{{Key: "child", Value: Record{Fields: []Field{{Key: "x", Value: Int(1)}, {Key: "y", Value: Int(2)}}}}}},
			b:    Record{Fields: []Field{{Key: "child", Value: Record{Fields: []Field{{Key: "y", Value: Int(2)}, {Key: "x", Value: Int(1)}}}}}},
			want: true,
		},
		{
			name: "nested_record_deep_value_mismatch",
			a:    Record{Fields: []Field{{Key: "child", Value: Record{Fields: []Field{{Key: "x", Value: Int(1)}}}}}},
			b:    Record{Fields: []Field{{Key: "child", Value: Record{Fields: []Field{{Key: "x", Value: Int(2)}}}}}},
			want: false,
		},
		{
			name: "nested_record_deep_type_mismatch",
			a:    Record{Fields: []Field{{Key: "child", Value: Record{Fields: []Field{{Key: "x", Value: Int(1)}}}}}},
			b:    Record{Fields: []Field{{Key: "child", Value: Record{Fields: []Field{{Key: "x", Value: Decimal("1.0")}}}}}},
			want: false,
		},
		{
			name: "nested_multi_level_collection_and_record",
			a: Collection{Elems: []Elem{
				{
					Label: "service",
					Body: Record{Fields: []Field{
						{
							Key: "containers",
							Value: Collection{Elems: []Elem{
								{Label: "app", Body: Record{Fields: []Field{{Key: "image", Value: String("web:1.0")}, {Key: "port", Value: Int(80)}}}},
							}},
						},
					}},
				},
			}},
			b: Collection{Elems: []Elem{
				{
					Label: "service",
					Body: Record{Fields: []Field{
						{
							Key: "containers",
							Value: Collection{Elems: []Elem{
								{Label: "app", Body: Record{Fields: []Field{{Key: "port", Value: Int(80)}, {Key: "image", Value: String("web:1.0")}}}},
							}},
						},
					}},
				},
			}},
			want: true,
		},
		{
			name: "nested_record_list_of_records_same_order_reordered_fields",
			a: Record{Fields: []Field{
				{
					Key: "items",
					Value: List{
						Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(2)}}},
						Record{Fields: []Field{{Key: "c", Value: Int(3)}, {Key: "d", Value: Int(4)}}},
					},
				},
			}},
			b: Record{Fields: []Field{
				{
					Key: "items",
					Value: List{
						Record{Fields: []Field{{Key: "b", Value: Int(2)}, {Key: "a", Value: Int(1)}}},
						Record{Fields: []Field{{Key: "d", Value: Int(4)}, {Key: "c", Value: Int(3)}}},
					},
				},
			}},
			want: true,
		},
		{
			name: "nested_record_list_of_records_swapped_list_order",
			a: Record{Fields: []Field{
				{
					Key: "items",
					Value: List{
						Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(2)}}},
						Record{Fields: []Field{{Key: "c", Value: Int(3)}, {Key: "d", Value: Int(4)}}},
					},
				},
			}},
			b: Record{Fields: []Field{
				{
					Key: "items",
					Value: List{
						Record{Fields: []Field{{Key: "c", Value: Int(3)}, {Key: "d", Value: Int(4)}}},
						Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(2)}}},
					},
				},
			}},
			want: false,
		},
		{
			name: "nested_list_of_lists_order_sensitive",
			a:    List{List{Int(1), Int(2)}, List{Int(3), Int(4)}},
			b:    List{List{Int(2), Int(1)}, List{Int(3), Int(4)}},
			want: false,
		},
		{
			name: "nested_all_types_identical",
			a:    valueBuildRecordAllTypes(),
			b:    valueBuildRecordAllTypes(),
			want: true,
		},
		{
			name: "nested_all_types_reordered_fields",
			a:    valueBuildRecordAllTypes(),
			b: Record{
				Fields: []Field{
					{Key: "coll", Value: Collection{Elems: []Elem{{Label: "c1", Body: Record{Fields: []Field{{Key: "flag", Value: Bool(true)}}}}}}},
					{Key: "rec", Value: Record{Fields: []Field{{Key: "inner", Value: String("ok")}}}},
					{Key: "list", Value: List{Int(1), Int(2)}},
					{Key: "ref", Value: Ref{"sys", "clock"}},
					{Key: "qty", Value: Quantity{Number: "100", Unit: "ms"}},
					{Key: "enum", Value: Enum("ready")},
					{Key: "bool", Value: Bool(true)},
					{Key: "dec", Value: Decimal("3.14")},
					{Key: "int", Value: Int(42)},
					{Key: "str", Value: String("val")},
				},
			},
			want: true,
		},
		{
			name: "nested_all_types_leaf_mutation_scalar",
			a:    valueBuildRecordAllTypes(),
			b: Record{
				Fields: []Field{
					{Key: "str", Value: String("val_modified")},
					{Key: "int", Value: Int(42)},
					{Key: "dec", Value: Decimal("3.14")},
					{Key: "bool", Value: Bool(true)},
					{Key: "enum", Value: Enum("ready")},
					{Key: "qty", Value: Quantity{Number: "100", Unit: "ms"}},
					{Key: "ref", Value: Ref{"sys", "clock"}},
					{Key: "list", Value: List{Int(1), Int(2)}},
					{Key: "rec", Value: Record{Fields: []Field{{Key: "inner", Value: String("ok")}}}},
					{Key: "coll", Value: Collection{Elems: []Elem{{Label: "c1", Body: Record{Fields: []Field{{Key: "flag", Value: Bool(true)}}}}}}},
				},
			},
			want: false,
		},
		{
			name: "nested_all_types_leaf_mutation_quantity_unit_casing",
			a:    valueBuildRecordAllTypes(),
			b: Record{
				Fields: []Field{
					{Key: "str", Value: String("val")},
					{Key: "int", Value: Int(42)},
					{Key: "dec", Value: Decimal("3.14")},
					{Key: "bool", Value: Bool(true)},
					{Key: "enum", Value: Enum("ready")},
					{Key: "qty", Value: Quantity{Number: "100", Unit: "Ms"}},
					{Key: "ref", Value: Ref{"sys", "clock"}},
					{Key: "list", Value: List{Int(1), Int(2)}},
					{Key: "rec", Value: Record{Fields: []Field{{Key: "inner", Value: String("ok")}}}},
					{Key: "coll", Value: Collection{Elems: []Elem{{Label: "c1", Body: Record{Fields: []Field{{Key: "flag", Value: Bool(true)}}}}}}},
				},
			},
			want: false,
		},
		{
			name: "nested_all_types_leaf_mutation_ref_segment",
			a:    valueBuildRecordAllTypes(),
			b: Record{
				Fields: []Field{
					{Key: "str", Value: String("val")},
					{Key: "int", Value: Int(42)},
					{Key: "dec", Value: Decimal("3.14")},
					{Key: "bool", Value: Bool(true)},
					{Key: "enum", Value: Enum("ready")},
					{Key: "qty", Value: Quantity{Number: "100", Unit: "ms"}},
					{Key: "ref", Value: Ref{"Sys", "clock"}},
					{Key: "list", Value: List{Int(1), Int(2)}},
					{Key: "rec", Value: Record{Fields: []Field{{Key: "inner", Value: String("ok")}}}},
					{Key: "coll", Value: Collection{Elems: []Elem{{Label: "c1", Body: Record{Fields: []Field{{Key: "flag", Value: Bool(true)}}}}}}},
				},
			},
			want: false,
		},
		{
			name: "nested_all_types_leaf_mutation_list_order",
			a:    valueBuildRecordAllTypes(),
			b: Record{
				Fields: []Field{
					{Key: "str", Value: String("val")},
					{Key: "int", Value: Int(42)},
					{Key: "dec", Value: Decimal("3.14")},
					{Key: "bool", Value: Bool(true)},
					{Key: "enum", Value: Enum("ready")},
					{Key: "qty", Value: Quantity{Number: "100", Unit: "ms"}},
					{Key: "ref", Value: Ref{"sys", "clock"}},
					{Key: "list", Value: List{Int(2), Int(1)}},
					{Key: "rec", Value: Record{Fields: []Field{{Key: "inner", Value: String("ok")}}}},
					{Key: "coll", Value: Collection{Elems: []Elem{{Label: "c1", Body: Record{Fields: []Field{{Key: "flag", Value: Bool(true)}}}}}}},
				},
			},
			want: false,
		},
	}
	valueRunTests(t, tests)
}

func TestEqual_SymmetryAndReflexivity(t *testing.T) {
	rec1 := Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "b", Value: Int(2)}}}
	rec2 := Record{Fields: []Field{{Key: "b", Value: Int(2)}, {Key: "a", Value: Int(1)}}}
	coll1 := Collection{Elems: []Elem{{Label: "web", Body: rec1}, {Label: "db", Body: Record{}}}}
	coll2 := Collection{Elems: []Elem{{Label: "db", Body: Record{}}, {Label: "web", Body: rec2}}}

	samples := []Value{
		nil,
		String(""),
		String("hello"),
		Int(0),
		Int(42),
		Int(-42),
		Decimal("0.0"),
		Decimal("1.25"),
		Bool(true),
		Bool(false),
		Enum("tcp"),
		Enum("udp"),
		Quantity{Number: "512", Unit: "Mi"},
		Quantity{Number: "0.25", Unit: "s"},
		Ref{},
		Ref(nil),
		Ref{"secret", "db"},
		List{},
		List(nil),
		List{Int(1), Int(2)},
		Record{},
		Record{Fields: nil},
		rec1,
		Collection{},
		Collection{Elems: nil},
		coll1,
	}

	for i, v := range samples {
		got := Equal(v, v)
		if !got {
			t.Errorf("reflexivity failed at index %d: Equal(%#v, %#v) = false, want true", i, v, v)
		}
	}

	for i := 0; i < len(samples); i++ {
		for j := 0; j < len(samples); j++ {
			ab := Equal(samples[i], samples[j])
			ba := Equal(samples[j], samples[i])
			if ab != ba {
				t.Errorf("symmetry failed: Equal(%#v, %#v) = %v, but Equal(%#v, %#v) = %v", samples[i], samples[j], ab, samples[j], samples[i], ba)
			}
		}
	}

	equivalentPairs := [][2]Value{
		{List{}, List(nil)},
		{Ref{}, Ref(nil)},
		{Record{Fields: []Field{}}, Record{Fields: nil}},
		{Collection{Elems: []Elem{}}, Collection{Elems: nil}},
		{rec1, rec2},
		{coll1, coll2},
	}
	for i, pair := range equivalentPairs {
		ab := Equal(pair[0], pair[1])
		ba := Equal(pair[1], pair[0])
		if !ab || !ba {
			t.Errorf("equivalent pair index %d failed: Equal(a, b) = %v, Equal(b, a) = %v, want both true", i, ab, ba)
		}
	}

	nonEquivalentPairs := [][2]Value{
		{Int(1), Decimal("1.0")},
		{String("tcp"), Enum("tcp")},
		{Quantity{Number: "512", Unit: "Mi"}, Quantity{Number: "512", Unit: "mi"}},
		{Ref{"a", "b"}, Ref{"b", "a"}},
		{List{Int(1), Int(2)}, List{Int(2), Int(1)}},
	}
	for i, pair := range nonEquivalentPairs {
		ab := Equal(pair[0], pair[1])
		ba := Equal(pair[1], pair[0])
		if ab || ba {
			t.Errorf("non-equivalent pair index %d failed: Equal(a, b) = %v, Equal(b, a) = %v, want both false", i, ab, ba)
		}
	}
}

func TestEqual_NoPanic(t *testing.T) {
	values := []Value{
		nil,
		String(""),
		String("test"),
		Int(0),
		Int(100),
		Decimal(""),
		Decimal("0.0"),
		Bool(false),
		Bool(true),
		Enum(""),
		Enum("tcp"),
		Quantity{},
		Quantity{Number: "1", Unit: "m"},
		Ref(nil),
		Ref{},
		Ref{"a", "b"},
		List(nil),
		List{},
		List{Int(1)},
		List{nil},
		Record{Fields: nil},
		Record{Fields: []Field{}},
		Record{Fields: []Field{{Key: "a", Value: nil}}},
		Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
		Record{Fields: []Field{{Key: "a", Value: Int(1)}, {Key: "a", Value: Int(2)}}},
		Collection{Elems: nil},
		Collection{Elems: []Elem{}},
		Collection{Elems: []Elem{{Label: "x", Body: Record{}}}},
		Collection{Elems: []Elem{{Label: "x", Body: Record{}}, {Label: "x", Body: Record{}}}},
	}

	for i := 0; i < len(values); i++ {
		for j := 0; j < len(values); j++ {
			a := values[i]
			b := values[j]
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Equal panicked on (%#v, %#v): %v", a, b, r)
					}
				}()
				_ = Equal(a, b)
			}()
		}
	}
}
