package ast

import (
	"testing"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/token"
)

func TestNodePos(t *testing.T) {
	s := &ScalarNode{Tok: token.Token{Pos: diag.Pos{Line: 1, Col: 4}}}
	l := &ListNode{LBrack: diag.Pos{Line: 2, Col: 8}}
	r := &RawNode{Pos: diag.Pos{Line: 3, Col: 2}}

	if s.NodePos() != (diag.Pos{Line: 1, Col: 4}) {
		t.Errorf("ScalarNode.NodePos() = %v", s.NodePos())
	}
	if l.NodePos() != (diag.Pos{Line: 2, Col: 8}) {
		t.Errorf("ListNode.NodePos() = %v", l.NodePos())
	}
	if r.NodePos() != (diag.Pos{Line: 3, Col: 2}) {
		t.Errorf("RawNode.NodePos() = %v", r.NodePos())
	}
}

func TestCloneEntryAndWithEntriesPure(t *testing.T) {
	if CloneEntry(nil) != nil {
		t.Fatalf("CloneEntry(nil) should be nil")
	}

	origEnt := &Entry{
		Kind:     EntryAssign,
		Key:      "k",
		Comments: []Comment{{Text: "c1"}},
	}
	clonedEnt := CloneEntry(origEnt)
	clonedEnt.Key = "k2"
	clonedEnt.Comments[0].Text = "mutated"

	if origEnt.Key != "k" || origEnt.Comments[0].Text != "c1" {
		t.Fatalf("CloneEntry did not isolate Comments slice from original")
	}

	origBlk := &Block{
		Entries:  []*Entry{origEnt},
		Trailing: []Comment{{Text: "trail"}},
		OneLine:  true,
		LBrace:   diag.Pos{Line: 1, Col: 1},
		RBrace:   diag.Pos{Line: 1, Col: 10},
	}
	newBlk := WithEntries(origBlk, []*Entry{clonedEnt})
	newBlk.Trailing[0].Text = "mutated-trail"

	if origBlk.Trailing[0].Text != "trail" || len(origBlk.Entries) != 1 || origBlk.Entries[0] != origEnt {
		t.Fatalf("WithEntries mutated original block")
	}
}

func TestScalarFromToken(t *testing.T) {
	cases := []struct {
		tok  token.Token
		want model.Value
	}{
		{token.Token{Kind: token.String, Text: "hi"}, model.String("hi")},
		{token.Token{Kind: token.Int, Text: "-42"}, model.Int(-42)},
		{token.Token{Kind: token.Decimal, Text: "1.200"}, model.Decimal("1.2")},
		{token.Token{Kind: token.Quantity, Text: "1.50s"}, model.Quantity{Number: "1.5", Unit: "s"}},
		{token.Token{Kind: token.Bool, Text: "true"}, model.Bool(true)},
		{token.Token{Kind: token.Bool, Text: "false"}, model.Bool(false)},
		{token.Token{Kind: token.Enum, Text: "tcp"}, model.Enum("tcp")},
		{token.Token{Kind: token.Ref, Segs: []string{"a", "b"}}, model.Ref{"a", "b"}},
	}
	for _, tc := range cases {
		node := ScalarFromToken(tc.tok)
		if !model.Equal(node.Val, tc.want) {
			t.Errorf("ScalarFromToken(%+v).Val = %#v, want %#v", tc.tok, node.Val, tc.want)
		}
	}
}

func TestBuildAndConvertRoundTrip(t *testing.T) {
	if got := BuildRecord(nil); len(got.Fields) != 0 {
		t.Errorf("BuildRecord(nil) should be empty, got %+v", got)
	}
	if got := BuildValue(nil); got != nil {
		t.Errorf("BuildValue(nil) = %v, want nil", got)
	}

	rawVal := BuildValue(&RawNode{Lines: []string{"line1", "line2"}})
	if rawVal != model.String("line1\nline2") {
		t.Errorf("BuildValue(RawNode) = %#v", rawVal)
	}

	rec := model.Record{
		Fields: []model.Field{
			{Key: "str", Value: model.String("hello")},
			{Key: "ref", Value: model.Ref{"secret", "key"}},
			{Key: "list", Value: model.List{model.Ref{"x", "y"}, model.Ref{"z"}}},
			{Key: "nested", Value: model.Record{
				Fields: []model.Field{{Key: "port", Value: model.Int(8080)}},
			}},
			{Key: "route", Value: model.Collection{
				Elems: []model.Elem{
					{Label: "a", Body: model.Record{Fields: []model.Field{{Key: "ok", Value: model.Bool(true)}}}},
					{Label: "b", Body: model.Record{Fields: []model.Field{{Key: "ok", Value: model.Bool(false)}}}},
				},
			}},
		},
	}

	blk := BlockFromRecord(rec)
	got := BuildRecord(blk)
	if !model.Equal(got, rec) {
		t.Fatalf("BlockFromRecord -> BuildRecord mismatch:\ngot:  %#v\nwant: %#v", got, rec)
	}

	// Verify Ref cloning in BuildValue (mutating returned Ref does not mutate AST node).
	refOut := got.Fields[1].Value.(model.Ref)
	refOut[0] = "mutated"
	listOut := got.Fields[2].Value.(model.List)
	listOut[0].(model.Ref)[0] = "mutated"

	gotAgain := BuildRecord(blk)
	if !model.Equal(gotAgain, rec) {
		t.Fatalf("mutating BuildRecord output mutated underlying AST")
	}
}
