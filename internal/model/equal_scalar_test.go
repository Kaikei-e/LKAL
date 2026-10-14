package model

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
				if Equal(s1.val, s2.val) {
					t.Errorf("Equal(%#v, %#v) = true, want false", s1.val, s2.val)
				}
			})
			t.Run(s2.name+"_vs_"+s1.name, func(t *testing.T) {
				if Equal(s2.val, s1.val) {
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
