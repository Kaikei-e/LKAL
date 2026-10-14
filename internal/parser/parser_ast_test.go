package parser

import (
	"reflect"
	"testing"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
)

func TestParse_VersionAndAST(t *testing.T) {
	t.Run("version_and_head", func(t *testing.T) {
		src := `### Header line 1
### Header line 2

@lkal "0.1"
`
		f := parserMustParse(t, "version_head.lkal", src)
		if f.Version != "0.1" {
			t.Errorf("Version = %q, want %q", f.Version, "0.1")
		}
		if f.VersionPos != (diag.Pos{Line: 4, Col: 1}) {
			t.Errorf("VersionPos = %v, want 4:1", f.VersionPos)
		}
		wantHead := []ast.Comment{
			{Pos: diag.Pos{Line: 1, Col: 1}, Text: "Header line 1", BlankBefore: false},
			{Pos: diag.Pos{Line: 2, Col: 1}, Text: "Header line 2", BlankBefore: false},
		}
		if !reflect.DeepEqual(f.Head, wantHead) {
			t.Errorf("Head mismatch:\ngot:  %+v\nwant: %+v", f.Head, wantHead)
		}
	})

	t.Run("entry_comments_and_blank_before", func(t *testing.T) {
		src := `@lkal "0.1"

### Comment for a
a = 1

b = 2
c = 3
`
		f := parserMustParse(t, "blank_before.lkal", src)
		if f.Body == nil || len(f.Body.Entries) != 3 {
			t.Fatalf("expected 3 entries in body, got %+v", f.Body)
		}

		ea := f.Body.Entries[0]
		if ea.Key != "a" || ea.KeyPos != (diag.Pos{Line: 4, Col: 1}) {
			t.Errorf("ea mismatch: key=%q pos=%v", ea.Key, ea.KeyPos)
		}
		if ea.BlankBefore != false {
			t.Errorf("ea.BlankBefore = %v, want false", ea.BlankBefore)
		}
		wantComments := []ast.Comment{
			{Pos: diag.Pos{Line: 3, Col: 1}, Text: "Comment for a", BlankBefore: true},
		}
		if !reflect.DeepEqual(ea.Comments, wantComments) {
			t.Errorf("ea.Comments mismatch:\ngot:  %+v\nwant: %+v", ea.Comments, wantComments)
		}

		eb := f.Body.Entries[1]
		if eb.Key != "b" || eb.KeyPos != (diag.Pos{Line: 6, Col: 1}) {
			t.Errorf("eb mismatch: key=%q pos=%v", eb.Key, eb.KeyPos)
		}
		if eb.BlankBefore != true {
			t.Errorf("eb.BlankBefore = %v, want true", eb.BlankBefore)
		}
		if len(eb.Comments) != 0 {
			t.Errorf("eb.Comments should be empty, got %+v", eb.Comments)
		}

		ec := f.Body.Entries[2]
		if ec.Key != "c" || ec.KeyPos != (diag.Pos{Line: 7, Col: 1}) {
			t.Errorf("ec mismatch: key=%q pos=%v", ec.Key, ec.KeyPos)
		}
		if ec.BlankBefore != false {
			t.Errorf("ec.BlankBefore = %v, want false", ec.BlankBefore)
		}
	})

	t.Run("trailing_comments", func(t *testing.T) {
		src := `@lkal "0.1"

rec {
  x = 1
  ### Trailing inside record
}

### Trailing at end of document
`
		f := parserMustParse(t, "trailing.lkal", src)
		if f.Body == nil || len(f.Body.Entries) != 1 {
			t.Fatalf("expected 1 entry in body, got %+v", f.Body)
		}
		recEntry := f.Body.Entries[0]
		if recEntry.Body == nil {
			t.Fatalf("recEntry.Body is nil")
		}
		wantRecTrailing := []ast.Comment{
			{Pos: diag.Pos{Line: 5, Col: 3}, Text: "Trailing inside record", BlankBefore: false},
		}
		if !reflect.DeepEqual(recEntry.Body.Trailing, wantRecTrailing) {
			t.Errorf("recEntry.Body.Trailing mismatch:\ngot:  %+v\nwant: %+v", recEntry.Body.Trailing, wantRecTrailing)
		}

		wantDocTrailing := []ast.Comment{
			{Pos: diag.Pos{Line: 8, Col: 1}, Text: "Trailing at end of document", BlankBefore: true},
		}
		if !reflect.DeepEqual(f.Body.Trailing, wantDocTrailing) {
			t.Errorf("f.Body.Trailing mismatch:\ngot:  %+v\nwant: %+v", f.Body.Trailing, wantDocTrailing)
		}
	})

	t.Run("one_line_flags_and_braces", func(t *testing.T) {
		src := `@lkal "0.1"

rec_one { a = 1 }
rec_multi {
  a = 1
}
list_one = [1, 2]
list_multi = [
  1,
  2,
]
`
		f := parserMustParse(t, "oneline.lkal", src)
		if f.Body == nil || len(f.Body.Entries) != 4 {
			t.Fatalf("expected 4 entries in body, got %+v", f.Body)
		}
		if f.Body.OneLine != false || f.Body.LBrace != (diag.Pos{}) || f.Body.RBrace != (diag.Pos{}) {
			t.Errorf("top-level body mismatch: oneLine=%v lbrace=%v rbrace=%v", f.Body.OneLine, f.Body.LBrace, f.Body.RBrace)
		}

		e0 := f.Body.Entries[0]
		if e0.Body == nil {
			t.Fatalf("rec_one body is nil")
		}
		if !e0.Body.OneLine {
			t.Errorf("rec_one oneLine = false, want true")
		}
		if e0.Body.LBrace != (diag.Pos{Line: 3, Col: 9}) || e0.Body.RBrace != (diag.Pos{Line: 3, Col: 17}) {
			t.Errorf("rec_one braces mismatch: lbrace=%v rbrace=%v, want 3:9, 3:17", e0.Body.LBrace, e0.Body.RBrace)
		}

		e1 := f.Body.Entries[1]
		if e1.Body == nil {
			t.Fatalf("rec_multi body is nil")
		}
		if e1.Body.OneLine {
			t.Errorf("rec_multi oneLine = true, want false")
		}
		if e1.Body.LBrace != (diag.Pos{Line: 4, Col: 11}) || e1.Body.RBrace != (diag.Pos{Line: 6, Col: 1}) {
			t.Errorf("rec_multi braces mismatch: lbrace=%v rbrace=%v, want 4:11, 6:1", e1.Body.LBrace, e1.Body.RBrace)
		}

		e2 := f.Body.Entries[2]
		l1, ok1 := e2.Value.(*ast.ListNode)
		if !ok1 {
			t.Fatalf("list_one value is %T, want *ast.ListNode", e2.Value)
		}
		if !l1.OneLine || l1.LBrack != (diag.Pos{Line: 7, Col: 12}) || l1.RBrack != (diag.Pos{Line: 7, Col: 17}) {
			t.Errorf("list_one mismatch: oneLine=%v lbrack=%v rbrack=%v, want true, 7:12, 7:17", l1.OneLine, l1.LBrack, l1.RBrack)
		}
		if len(l1.Elems) != 2 {
			t.Errorf("list_one len(elems) = %d, want 2", len(l1.Elems))
		}

		e3 := f.Body.Entries[3]
		l2, ok2 := e3.Value.(*ast.ListNode)
		if !ok2 {
			t.Fatalf("list_multi value is %T, want *ast.ListNode", e3.Value)
		}
		if l2.OneLine || l2.LBrack != (diag.Pos{Line: 8, Col: 14}) || l2.RBrack != (diag.Pos{Line: 11, Col: 1}) {
			t.Errorf("list_multi mismatch: oneLine=%v lbrack=%v rbrack=%v, want false, 8:14, 11:1", l2.OneLine, l2.LBrack, l2.RBrack)
		}
		if len(l2.Elems) != 2 {
			t.Errorf("list_multi len(elems) = %d, want 2", len(l2.Elems))
		}
	})

	t.Run("entry_kinds_and_positions", func(t *testing.T) {
		src := `@lkal "0.1"

assign = "val"
record { x = 1 }
elem "label1" { y = 2 }
`
		f := parserMustParse(t, "kinds.lkal", src)
		if f.Body == nil || len(f.Body.Entries) != 3 {
			t.Fatalf("expected 3 entries in body, got %+v", f.Body)
		}

		e0 := f.Body.Entries[0]
		if e0.Kind != ast.EntryAssign || e0.Key != "assign" || e0.KeyPos != (diag.Pos{Line: 3, Col: 1}) {
			t.Errorf("e0 assign mismatch: kind=%v key=%q pos=%v", e0.Kind, e0.Key, e0.KeyPos)
		}
		if e0.Body != nil {
			t.Errorf("entryAssign body must be nil, got %v", e0.Body)
		}
		sn, ok := e0.Value.(*ast.ScalarNode)
		if !ok {
			t.Fatalf("e0 value is %T, want *ast.ScalarNode", e0.Value)
		}
		if !model.Equal(sn.Val, model.String("val")) {
			t.Errorf("e0 value mismatch: %+v", sn.Val)
		}
		if sn.NodePos() != (diag.Pos{Line: 3, Col: 10}) {
			t.Errorf("e0 NodePos mismatch: got %v want 3:10", sn.NodePos())
		}

		e1 := f.Body.Entries[1]
		if e1.Kind != ast.EntryRecord || e1.Key != "record" || e1.KeyPos != (diag.Pos{Line: 4, Col: 1}) {
			t.Errorf("e1 record mismatch: kind=%v key=%q pos=%v", e1.Kind, e1.Key, e1.KeyPos)
		}
		if e1.Label != "" || e1.LabelPos != (diag.Pos{}) {
			t.Errorf("entryRecord label should be empty, got label=%q pos=%v", e1.Label, e1.LabelPos)
		}
		if e1.Value != nil || e1.Body == nil {
			t.Errorf("entryRecord value must be nil and body non-nil")
		}

		e2 := f.Body.Entries[2]
		if e2.Kind != ast.EntryElem || e2.Key != "elem" || e2.KeyPos != (diag.Pos{Line: 5, Col: 1}) {
			t.Errorf("e2 elem mismatch: kind=%v key=%q pos=%v", e2.Kind, e2.Key, e2.KeyPos)
		}
		if e2.Label != "label1" || e2.LabelPos != (diag.Pos{Line: 5, Col: 6}) {
			t.Errorf("e2 elem label mismatch: label=%q pos=%v, want label1, 5:6", e2.Label, e2.LabelPos)
		}
		if e2.Value != nil || e2.Body == nil {
			t.Errorf("entryElem value must be nil and body non-nil")
		}
	})

	t.Run("spec_full_example_ast", func(t *testing.T) {
		src := `@lkal "0.1"

### フロントエンドのWebサーバー
service "web" {
  replicas = 3
  traffic-share = 0.25
  labels { "app.kubernetes.io/name" = "web", tier = "frontend" }

  container "app" {
    image = "ghcr.io/acme/web:1.4.2"
    args = ["--port", "8080"]
    memory = 512Mi
    cpu = 250m
    timeout = 30s
    port "http" { number = 8080, protocol = .tcp }
    port "metrics" { number = 9090, protocol = .tcp }
    env { LOG_LEVEL = "info" }
    secret = &secret/db-creds
  }

  ### 起動スクリプト。各行を \\ で始める
  startup =
    \\#!/bin/sh
    \\exec /app/web --port 8080
}
`
		f := parserMustParse(t, "spec_ast.lkal", src)
		if f.Body == nil || len(f.Body.Entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", f.Body)
		}
		svc := f.Body.Entries[0]
		if svc.Kind != ast.EntryElem || svc.Key != "service" || svc.Label != "web" {
			t.Errorf("service entry mismatch: kind=%v key=%q label=%q", svc.Kind, svc.Key, svc.Label)
		}
		if len(svc.Comments) != 1 || svc.Comments[0].Text != "フロントエンドのWebサーバー" {
			t.Errorf("service comments mismatch: %+v", svc.Comments)
		}
		if svc.Body == nil || len(svc.Body.Entries) != 5 {
			t.Fatalf("expected 5 entries in service body, got %+v", svc.Body)
		}

		labels := svc.Body.Entries[2]
		if labels.Key != "labels" || labels.Body == nil {
			t.Fatalf("labels entry invalid or body is nil: %+v", labels)
		}
		if !labels.Body.OneLine {
			t.Errorf("labels oneLine = false, want true")
		}

		container := svc.Body.Entries[3]
		if container.Key != "container" || container.Label != "app" || !container.BlankBefore {
			t.Errorf("container blankBefore = %v, want true", container.BlankBefore)
		}
		if container.Body == nil || len(container.Body.Entries) != 9 {
			t.Fatalf("expected 9 entries in container, got %+v", container.Body)
		}
		args := container.Body.Entries[1]
		argsList, ok := args.Value.(*ast.ListNode)
		if !ok {
			t.Fatalf("args value is %T, want *ast.ListNode", args.Value)
		}
		if !argsList.OneLine || len(argsList.Elems) != 2 {
			t.Errorf("args list mismatch: oneLine=%v len=%d", argsList.OneLine, len(argsList.Elems))
		}

		startup := svc.Body.Entries[4]
		if startup.Key != "startup" || startup.BlankBefore != false {
			t.Errorf("startup blankBefore = %v, want false", startup.BlankBefore)
		}
		if len(startup.Comments) != 1 || startup.Comments[0].Text != `起動スクリプト。各行を \\ で始める` || startup.Comments[0].BlankBefore != true {
			t.Errorf("startup comments mismatch: %+v", startup.Comments)
		}
		startupRaw, ok := startup.Value.(*ast.RawNode)
		if !ok {
			t.Fatalf("startup value is %T, want *ast.RawNode", startup.Value)
		}
		wantStartupLines := []string{"#!/bin/sh", "exec /app/web --port 8080"}
		if !reflect.DeepEqual(startupRaw.Lines, wantStartupLines) {
			t.Errorf("startup lines mismatch: got %v want %v", startupRaw.Lines, wantStartupLines)
		}
		if startupRaw.Pos != (diag.Pos{Line: 23, Col: 5}) {
			t.Errorf("startup raw pos mismatch: got %v want 23:5", startupRaw.Pos)
		}
	})
}
