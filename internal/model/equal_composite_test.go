package model

import "testing"

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
