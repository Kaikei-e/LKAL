package model

import "testing"

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
		if !Equal(v, v) {
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
