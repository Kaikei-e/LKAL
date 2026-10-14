package model

import (
	"strings"
	"testing"

	"github.com/Kaikei-e/LKAL/internal/diag"
)

func TestScalarTypeAndMarkerMethods(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{String("a"), "String"},
		{Int(1), "Int"},
		{Decimal("1.0"), "Decimal"},
		{Bool(true), "Bool"},
		{Enum("tcp"), "Enum"},
		{Quantity{Number: "5", Unit: "s"}, "Quantity"},
		{Ref{"a"}, "Ref"},
		{List{}, ""},
		{Record{}, ""},
		{Collection{}, ""},
		{nil, ""},
	}
	for _, tc := range cases {
		if tc.v != nil {
			tc.v.lkalValue()
		}
		if got := ScalarType(tc.v); got != tc.want {
			t.Errorf("ScalarType(%T) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestValidateValue(t *testing.T) {
	d101 := strings.Repeat("1", 100) + ".1"
	badUTF8 := string([]byte{0xff})

	validCases := []Value{
		String("hello"),
		Int(42),
		Decimal("0.25"),
		Bool(false),
		Enum("tcp"),
		Quantity{Number: "512", Unit: "Mi"},
		Ref{"a", "b"},
		List{Int(1), Int(2)},
		Record{Fields: []Field{{Key: "k", Value: Int(1)}}},
		Collection{Elems: []Elem{{Label: "e1", Body: Record{}}}},
	}
	for _, v := range validCases {
		if hint := ValidateValue(v, 1); hint != "" {
			t.Errorf("ValidateValue(%#v) = %q, want empty", v, hint)
		}
	}

	invalidCases := []struct {
		name     string
		v        Value
		depth    int
		wantHint string
	}{
		{"Nil", nil, 1, "use a non-nil value"},
		{"BadUTF8String", String(badUTF8), 1, "use valid UTF-8 text"},
		{"IntAsDecimal", Decimal("42"), 1, "write a decimal such as 0.25"},
		{"DecimalTooLong", Decimal(d101), 1, "keep numbers within 100 digits"},
		{"QuantityNoUnit", Quantity{Number: "10", Unit: ""}, 1, "write a quantity such as 512Mi"},
		{"QuantityDigitInUnit", Quantity{Number: "10", Unit: "m2"}, 1, "write a quantity such as 512Mi"},
		{"QuantityTooLong", Quantity{Number: d101, Unit: "s"}, 1, "keep numbers within 100 digits"},
		{"EnumBadUTF8", Enum(badUTF8), 1, "use valid UTF-8 text"},
		{"EnumBadStart", Enum("1abc"), 1, "write an enum name that starts with a letter or _"},
		{"RefEmpty", Ref{}, 1, "write a reference with at least one segment"},
		{"RefBadUTF8", Ref{badUTF8}, 1, "use valid UTF-8 text"},
		{"ListNonScalar", List{Record{}}, 1, "use only scalars as list elements"},
		{"ListInvalidElem", List{Decimal("bad")}, 1, "write a decimal such as 0.25"},
		{"ListMixedTypes", List{Int(1), String("a")}, 1, "use one type for all list elements"},
		{"RecordDepthExceeded", Record{}, diag.MaxBlockDepth + 1, diag.DefaultHint("L107")},
		{"RecordKeyBadUTF8", Record{Fields: []Field{{Key: badUTF8, Value: Int(1)}}}, 1, "use valid UTF-8 text"},
		{"RecordKeyDollar", Record{Fields: []Field{{Key: "$k", Value: Int(1)}}}, 1, "rename keys that start with $"},
		{"RecordDupKey", Record{Fields: []Field{{Key: "k", Value: Int(1)}, {Key: "k", Value: Int(2)}}}, 1, "remove the duplicate key"},
		{"RecordInvalidChild", Record{Fields: []Field{{Key: "k", Value: Ref{}}}}, 1, "write a reference with at least one segment"},
		{"CollectionEmpty", Collection{}, 1, "write at least one collection element"},
		{"CollectionBadUTF8Label", Collection{Elems: []Elem{{Label: badUTF8, Body: Record{}}}}, 1, "use valid UTF-8 text"},
		{"CollectionDupLabel", Collection{Elems: []Elem{{Label: "a", Body: Record{}}, {Label: "a", Body: Record{}}}}, 1, "remove the duplicate label"},
		{"CollectionInvalidBody", Collection{Elems: []Elem{{Label: "a", Body: Record{Fields: []Field{{Key: "$x", Value: Int(1)}}}}}}, 1, "rename keys that start with $"},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidateValue(tc.v, tc.depth); got != tc.wantHint {
				t.Errorf("ValidateValue() = %q, want %q", got, tc.wantHint)
			}
		})
	}
}

func TestNormalizeValuePureAndImmutability(t *testing.T) {
	origRef := Ref{"a", "b"}
	origList := List{Decimal("1.500"), Quantity{Number: "-0.00", Unit: "s"}, origRef}
	origRec := Record{
		Fields: []Field{
			{Key: "list", Value: origList},
			{Key: "coll", Value: Collection{
				Elems: []Elem{
					{Label: "item", Body: Record{Fields: []Field{{Key: "d", Value: Decimal("-0.0")}}}},
				},
			}},
		},
	}

	norm := NormalizeValue(origRec).(Record)

	// Verify normalized values.
	normList := norm.Fields[0].Value.(List)
	if normList[0] != Decimal("1.5") {
		t.Errorf("got %v, want Decimal(\"1.5\")", normList[0])
	}
	if normList[1] != (Quantity{Number: "0.0", Unit: "s"}) {
		t.Errorf("got %v, want Quantity{0.0, s}", normList[1])
	}
	normColl := norm.Fields[1].Value.(Collection)
	if normColl.Elems[0].Body.Fields[0].Value != Decimal("0.0") {
		t.Errorf("got %v, want Decimal(\"0.0\")", normColl.Elems[0].Body.Fields[0].Value)
	}

	// Mutate normalized structures and verify original is untouched (pure deep copy).
	normRef := normList[2].(Ref)
	normRef[0] = "mutated"
	normList[0] = Int(999)
	norm.Fields[0].Key = "mutated"

	if origRef[0] != "a" || origList[0] != Decimal("1.500") || origRec.Fields[0].Key != "list" {
		t.Fatalf("NormalizeValue shared mutable backing slices with input")
	}
}
