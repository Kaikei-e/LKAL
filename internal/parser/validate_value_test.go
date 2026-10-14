package parser

import (
	"testing"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/token"
)

func TestValidateDirectOnAST(t *testing.T) {
	if errs := Validate("test.lkal", nil); errs != nil {
		t.Fatalf("Validate(nil) = %v, want nil", errs)
	}

	t.Run("L201DuplicateKeyAndLabel", func(t *testing.T) {
		blk := &ast.Block{
			Entries: []*ast.Entry{
				{Kind: ast.EntryAssign, Key: "a", KeyPos: diag.Pos{Line: 1, Col: 1}, Value: &ast.ScalarNode{Val: model.Int(1)}},
				{Kind: ast.EntryAssign, Key: "a", KeyPos: diag.Pos{Line: 2, Col: 1}, Value: &ast.ScalarNode{Val: model.Int(2)}},
				{Kind: ast.EntryElem, Key: "c", Label: "x", LabelPos: diag.Pos{Line: 3, Col: 3}, Body: &ast.Block{}},
				{Kind: ast.EntryElem, Key: "c", Label: "x", LabelPos: diag.Pos{Line: 4, Col: 3}, Body: &ast.Block{}},
			},
		}
		errs := Validate("test.lkal", blk)
		if len(errs) != 2 || errs[0].Code != "L201" || errs[1].Code != "L201" {
			t.Fatalf("expected two L201 errors, got %+v", errs)
		}
	})

	t.Run("L202MixedEntryKinds", func(t *testing.T) {
		blk := &ast.Block{
			Entries: []*ast.Entry{
				{Kind: ast.EntryAssign, Key: "a", KeyPos: diag.Pos{Line: 1, Col: 1}, Value: &ast.ScalarNode{Val: model.Int(1)}},
				{Kind: ast.EntryRecord, Key: "a", KeyPos: diag.Pos{Line: 2, Col: 1}, Body: &ast.Block{}},
			},
		}
		errs := Validate("test.lkal", blk)
		if len(errs) != 1 || errs[0].Code != "L202" {
			t.Fatalf("expected L202, got %+v", errs)
		}
	})

	t.Run("L203NonContiguousCollectionElements", func(t *testing.T) {
		blk := &ast.Block{
			Entries: []*ast.Entry{
				{Kind: ast.EntryElem, Key: "svc", Label: "a", KeyPos: diag.Pos{Line: 1, Col: 1}, Body: &ast.Block{}},
				{Kind: ast.EntryAssign, Key: "other", KeyPos: diag.Pos{Line: 2, Col: 1}, Value: &ast.ScalarNode{Val: model.Int(1)}},
				{Kind: ast.EntryElem, Key: "svc", Label: "b", KeyPos: diag.Pos{Line: 3, Col: 1}, Body: &ast.Block{}},
			},
		}
		errs := Validate("test.lkal", blk)
		if len(errs) != 1 || errs[0].Code != "L203" {
			t.Fatalf("expected L203, got %+v", errs)
		}
	})

	t.Run("L204ListMixesTypes", func(t *testing.T) {
		blk := &ast.Block{
			Entries: []*ast.Entry{
				{
					Kind: ast.EntryAssign,
					Key:  "items",
					Value: &ast.ListNode{
						Elems: []*ast.ScalarNode{
							{Tok: token.Token{Kind: token.Int, Pos: diag.Pos{Line: 1, Col: 10}}},
							{Tok: token.Token{Kind: token.String, Pos: diag.Pos{Line: 1, Col: 13}}},
						},
					},
				},
			},
		}
		errs := Validate("test.lkal", blk)
		if len(errs) != 1 || errs[0].Code != "L204" {
			t.Fatalf("expected L204, got %+v", errs)
		}
	})

	t.Run("L205KeyStartsWithDollar", func(t *testing.T) {
		blk := &ast.Block{
			Entries: []*ast.Entry{
				{Kind: ast.EntryAssign, Key: "$bad", KeyPos: diag.Pos{Line: 1, Col: 1}, Value: &ast.ScalarNode{Val: model.Int(1)}},
			},
		}
		errs := Validate("test.lkal", blk)
		if len(errs) != 1 || errs[0].Code != "L205" {
			t.Fatalf("expected L205, got %+v", errs)
		}
	})
}

func TestParseValueDirect(t *testing.T) {
	validCases := []struct {
		src  string
		want model.Value
	}{
		{`"hello"`, model.String("hello")},
		{"42", model.Int(42)},
		{"1.50", model.Decimal("1.5")},
		{"true", model.Bool(true)},
		{".tcp", model.Enum("tcp")},
		{"500ms", model.Quantity{Number: "500", Unit: "ms"}},
		{"&a/b", model.Ref{"a", "b"}},
		{"[1, 2, 3]", model.List{model.Int(1), model.Int(2), model.Int(3)}},
		{"\\\\line1\n\\\\line2", model.String("line1\nline2")},
		{"{ a = 1 }", model.Record{Fields: []model.Field{{Key: "a", Value: model.Int(1)}}}},
		{`"web" { port = 80 }`, model.Collection{Elems: []model.Elem{{Label: "web", Body: model.Record{Fields: []model.Field{{Key: "port", Value: model.Int(80)}}}}}}},
	}

	for _, tc := range validCases {
		got, err := ParseValue([]byte(tc.src))
		if err != nil {
			t.Fatalf("ParseValue(%q) unexpected error: %v", tc.src, err)
		}
		if !model.Equal(got, tc.want) {
			t.Errorf("ParseValue(%q) = %#v, want %#v", tc.src, got, tc.want)
		}
	}

	invalidSrcs := []string{
		"",
		"### only comment\n",
		"bareword",
		"1 2",
		`"web" { a = 1 } extra`,
	}
	for _, src := range invalidSrcs {
		if _, err := ParseValue([]byte(src)); err == nil {
			t.Errorf("ParseValue(%q) succeeded, expected error", src)
		}
	}
}
