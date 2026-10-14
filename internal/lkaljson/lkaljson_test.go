package lkaljson

import (
	"errors"
	"strings"
	"testing"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
)

func TestToJSONAndFromJSONRoundTrip(t *testing.T) {
	if _, err := ToJSON(nil, false); err == nil {
		t.Fatalf("ToJSON(nil) should return an error")
	}

	rec := model.Record{
		Fields: []model.Field{
			{Key: "str", Value: model.String("hi")},
			{Key: "num", Value: model.Int(42)},
			{Key: "dec", Value: model.Decimal("3.14")},
			{Key: "ok", Value: model.Bool(true)},
			{Key: "mode", Value: model.Enum("tcp")},
			{Key: "mem", Value: model.Quantity{Number: "512", Unit: "Mi"}},
			{Key: "target", Value: model.Ref{"svc", "db"}},
			{Key: "tags", Value: model.List{model.Enum("a"), model.Enum("b")}},
			{Key: "emptyObj", Value: model.Record{}},
			{Key: "nested", Value: model.Record{Fields: []model.Field{{Key: "x", Value: model.Int(1)}}}},
			{Key: "route", Value: model.Collection{
				Elems: []model.Elem{
					{Label: "main", Body: model.Record{Fields: []model.Field{{Key: "port", Value: model.Int(80)}}}},
				},
			}},
		},
	}
	f := &ast.File{Version: "0.1", Body: ast.BlockFromRecord(rec)}

	// Encode LKAL-JSON and decode back.
	encoded, err := ToJSON(f, false)
	if err != nil {
		t.Fatalf("ToJSON(plain=false) failed: %v", err)
	}
	decodedFile, err := FromJSON(encoded)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}
	gotRec := ast.BuildRecord(decodedFile.Body)
	if !model.Equal(gotRec, rec) {
		t.Errorf("round-trip record mismatch:\ngot:  %#v\nwant: %#v", gotRec, rec)
	}

	// Plain JSON mode omits $lkal and unwraps tags.
	plainBytes, err := ToJSON(f, true)
	if err != nil {
		t.Fatalf("ToJSON(plain=true) failed: %v", err)
	}
	plainStr := string(plainBytes)
	for _, marker := range []string{"$lkal", "$enum", "$qty", "$ref", "$coll"} {
		if strings.Contains(plainStr, marker) {
			t.Errorf("plain JSON should not contain %q:\n%s", marker, plainStr)
		}
	}
}

func TestFromJSONErrorCodes(t *testing.T) {
	d101 := strings.Repeat("9", 100) + ".9s"
	tests := []struct {
		name     string
		json     string
		wantCode string
	}{
		{"InvalidSyntax", `{`, "L301"},
		{"TrailingTokens", `{"$lkal": "0.1"} extra`, "L301"},
		{"TopNotObject", `["$lkal"]`, "L105"},
		{"TopEmptyObject", `{}`, "L105"},
		{"MissingVersion", `{"a": 1}`, "L105"},
		{"VersionNotString", `{"$lkal": 123}`, "L105"},
		{"RepeatedVersion", `{"$lkal": "0.1", "$lkal": "0.1"}`, "L105"},
		{"UnsupportedVersion", `{"$lkal": "0.2"}`, "L207"},
		{"NullValue", `{"$lkal": "0.1", "a": null}`, "L302"},
		{"NullInList", `{"$lkal": "0.1", "a": [null]}`, "L302"},
		{"NullInTag", `{"$lkal": "0.1", "a": {"$enum": null}}`, "L302"},
		{"NullInCollElem", `{"$lkal": "0.1", "a": {"$coll": {"x": null}}}`, "L302"},
		{"ExponentNumber", `{"$lkal": "0.1", "a": 1e5}`, "L303"},
		{"NumberOutOfRange", `{"$lkal": "0.1", "a": 9223372036854775808}`, "L005"},
		{"QtyOutOfRange", `{"$lkal": "0.1", "a": {"$qty": "` + d101 + `"}}`, "L005"},
		{"ListNonScalarObject", `{"$lkal": "0.1", "a": [{"x": 1}]}`, "L304"},
		{"ListNonScalarEmptyObject", `{"$lkal": "0.1", "a": [{}]}`, "L304"},
		{"ListNonScalarArray", `{"$lkal": "0.1", "a": [[1]]}`, "L304"},
		{"ListMixedTypes", `{"$lkal": "0.1", "a": [1, "two"]}`, "L204"},
		{"DuplicateKey", `{"$lkal": "0.1", "a": 1, "a": 2}`, "L201"},
		{"DuplicateCollLabel", `{"$lkal": "0.1", "a": {"$coll": {"x": {}, "x": {}}}}`, "L201"},
		{"DollarKey", `{"$lkal": "0.1", "$bad": 1}`, "L205"},
		{"MalformedEnumTag", `{"$lkal": "0.1", "a": {"$enum": "1bad"}}`, "L305"},
		{"MalformedEnumNonString", `{"$lkal": "0.1", "a": {"$enum": 123}}`, "L305"},
		{"MalformedQtyTag", `{"$lkal": "0.1", "a": {"$qty": "10"}}`, "L305"},
		{"MalformedRefTag", `{"$lkal": "0.1", "a": {"$ref": ""}}`, "L305"},
		{"MalformedCollNotObj", `{"$lkal": "0.1", "a": {"$coll": 123}}`, "L305"},
		{"MalformedCollEmpty", `{"$lkal": "0.1", "a": {"$coll": {}}}`, "L305"},
		{"MalformedCollElemNotObj", `{"$lkal": "0.1", "a": {"$coll": {"x": 1}}}`, "L305"},
		{"MalformedCollElemIsTag", `{"$lkal": "0.1", "a": {"$coll": {"x": {"$enum": "tcp"}}}}`, "L305"},
		{"MultiKeyTagObject", `{"$lkal": "0.1", "a": {"$enum": "tcp", "extra": 1}}`, "L305"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromJSON([]byte(tt.json))
			if err == nil {
				t.Fatalf("FromJSON(%s) succeeded, expected %s", tt.json, tt.wantCode)
			}
			var el *diag.ErrorList
			if !errors.As(err, &el) || len(el.Errors) == 0 {
				t.Fatalf("expected *diag.ErrorList, got %T: %v", err, err)
			}
			if el.Errors[0].Code != tt.wantCode {
				t.Errorf("got code %s, want %s (full: %v)", el.Errors[0].Code, tt.wantCode, err)
			}
		})
	}
}
