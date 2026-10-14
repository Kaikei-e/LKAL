package path

import (
	"errors"
	"testing"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/format"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/parser"
)

func mustParseBlock(t *testing.T, src string) (*ast.File, *ast.Block) {
	t.Helper()
	f, err := parser.Parse("test.lkal", []byte(src))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	return f, f.Body
}

func TestKeyRange(t *testing.T) {
	_, blk := mustParseBlock(t, "@lkal \"0.1\"\n\na = 1\nsvc \"x\" {}\nsvc \"y\" {}\nb = 2\n")
	if first, last := KeyRange(blk, "svc"); first != 1 || last != 2 {
		t.Errorf("KeyRange(\"svc\") = (%d, %d), want (1, 2)", first, last)
	}
	if first, last := KeyRange(blk, "missing"); first != -1 || last != -1 {
		t.Errorf("KeyRange(\"missing\") = (%d, %d), want (-1, -1)", first, last)
	}
}

func TestGetSetDeletePureImmutability(t *testing.T) {
	origSrc := `@lkal "0.1"

### keep comment
server {
  host = "localhost"
  port = 8080
  tags = [
    "a",
    "b",
  ]
}

inline { x = 1 }

route "api" {
  timeout = 5s
}
`
	f, origRoot := mustParseBlock(t, origSrc)
	origFormatted := string(format.FormatFile(f))

	// 1. Set an existing nested key.
	root1, err := Set("test.lkal", origRoot, "server/port", model.Int(9090))
	if err != nil {
		t.Fatalf("Set(server/port) failed: %v", err)
	}

	// Verify origRoot was NOT mutated at all.
	if gotOrig := string(format.FormatFile(&ast.File{Version: "0.1", Body: origRoot})); gotOrig != origFormatted {
		t.Fatalf("Set mutated original AST!\ngot:\n%s\nwant:\n%s", gotOrig, origFormatted)
	}

	// Verify new root has updated value while preserving sibling & comments.
	vPort, err := Get("test.lkal", root1, "server/port")
	if err != nil || !model.Equal(vPort, model.Int(9090)) {
		t.Fatalf("Get(server/port) on root1 = %v, %v; want 9090", vPort, err)
	}
	vOrigPort, err := Get("test.lkal", origRoot, "server/port")
	if err != nil || !model.Equal(vOrigPort, model.Int(8080)) {
		t.Fatalf("Get(server/port) on origRoot = %v, %v; want 8080", vOrigPort, err)
	}

	// 2. Replace existing list, existing inline record, and existing collection element.
	root1b, err := Set("test.lkal", root1, "server/tags", model.List{model.String("c")})
	if err != nil {
		t.Fatalf("Set(server/tags) failed: %v", err)
	}
	root1c, err := Set("test.lkal", root1b, "inline", model.Record{Fields: []model.Field{{Key: "x", Value: model.Int(2)}}})
	if err != nil {
		t.Fatalf("Set(inline) failed: %v", err)
	}
	root1d, err := Set("test.lkal", root1c, "route/api", model.Record{Fields: []model.Field{{Key: "timeout", Value: model.Quantity{Number: "2", Unit: "s"}}}})
	if err != nil {
		t.Fatalf("Set(route/api) failed: %v", err)
	}
	root1e, err := Set("test.lkal", root1d, "route/api/timeout", model.Quantity{Number: "3", Unit: "s"})
	if err != nil {
		t.Fatalf("Set(route/api/timeout) failed: %v", err)
	}

	// 3. Add a collection element on root1e -> root2.
	root2, err := Set("test.lkal", root1e, "route/admin", model.Record{
		Fields: []model.Field{{Key: "timeout", Value: model.Quantity{Number: "10", Unit: "s"}}},
	})
	if err != nil {
		t.Fatalf("Set(route/admin) failed: %v", err)
	}
	if _, err := Get("test.lkal", root1e, "route/admin"); err == nil {
		t.Fatalf("Set(route/admin) leaked into root1e")
	}
	vColl, err := Get("test.lkal", root2, "route")
	if err != nil || len(vColl.(model.Collection).Elems) != 2 {
		t.Fatalf("Get(route) on root2 = %#v, %v", vColl, err)
	}
	vServer, err := Get("test.lkal", root2, "server")
	if err != nil || len(vServer.(model.Record).Fields) != 3 {
		t.Fatalf("Get(server) on root2 = %#v, %v", vServer, err)
	}
	vElem, err := Get("test.lkal", root2, "route/api")
	if err != nil || len(vElem.(model.Record).Fields) != 1 {
		t.Fatalf("Get(route/api) on root2 = %#v, %v", vElem, err)
	}

	// 4. Set whole collection at a new key.
	root2b, err := Set("test.lkal", root2, "workers", model.Collection{
		Elems: []model.Elem{{Label: "w1", Body: model.Record{}}},
	})
	if err != nil {
		t.Fatalf("Set(workers) failed: %v", err)
	}

	// 5. Delete server/host on root2b -> root3 without mutating root2b.
	root3, err := Delete("test.lkal", root2b, "server/host")
	if err != nil {
		t.Fatalf("Delete(server/host) failed: %v", err)
	}
	if _, err := Get("test.lkal", root2b, "server/host"); err != nil {
		t.Fatalf("Delete(server/host) mutated root2b: %v", err)
	}
	if _, err := Get("test.lkal", root3, "server/host"); err == nil {
		t.Fatalf("server/host should be deleted in root3")
	}

	// 6. Delete collection element route/api on root3 -> root4.
	root4, err := Delete("test.lkal", root3, "route/api")
	if err != nil {
		t.Fatalf("Delete(route/api) failed: %v", err)
	}
	if _, err := Get("test.lkal", root3, "route/api"); err != nil {
		t.Fatalf("Delete(route/api) mutated root3: %v", err)
	}
	if _, err := Get("test.lkal", root4, "route/api"); err == nil {
		t.Fatalf("route/api should be deleted in root4")
	}

	// 7. Delete entry with BlankBefore to verify blank propagation.
	root5, err := Delete("test.lkal", root4, "inline")
	if err != nil {
		t.Fatalf("Delete(inline) failed: %v", err)
	}
	if _, err := Get("test.lkal", root5, "inline"); err == nil {
		t.Fatalf("inline should be deleted in root5")
	}
}

func TestPathErrors(t *testing.T) {
	_, root := mustParseBlock(t, "@lkal \"0.1\"\n\na = 1\nsvc \"web\" {\n  port = 80\n}\n")

	checkCode := func(t *testing.T, err error, want string) {
		t.Helper()
		if err == nil {
			t.Fatalf("expected error %s, got nil", want)
		}
		var el *diag.ErrorList
		if !errors.As(err, &el) || len(el.Errors) == 0 || el.Errors[0].Code != want {
			t.Fatalf("got error %v, want code %s", err, want)
		}
	}

	_, err := Get("test.lkal", root, "")
	checkCode(t, err, "L401")

	_, err = Get("test.lkal", root, "missing")
	checkCode(t, err, "L402")

	_, err = Get("test.lkal", root, "a/child")
	checkCode(t, err, "L402")

	_, err = Get("test.lkal", root, "a/b/c")
	checkCode(t, err, "L402")

	_, err = Get("test.lkal", root, "svc/missing")
	checkCode(t, err, "L402")

	_, err = Get("test.lkal", root, "svc/missing/port")
	checkCode(t, err, "L402")

	_, err = Set("test.lkal", root, "bad/", model.Int(1))
	checkCode(t, err, "L401")

	_, err = Set("test.lkal", root, `"$bad"`, model.Int(1))
	checkCode(t, err, "L403")

	_, err = Set("test.lkal", root, "k", model.Decimal("not-a-dec"))
	checkCode(t, err, "L403")

	_, err = Set("test.lkal", root, "svc/web", model.Int(1))
	checkCode(t, err, "L403")

	_, err = Set("test.lkal", root, "svc/web", model.Record{Fields: []model.Field{{Key: "$x", Value: model.Int(1)}}})
	checkCode(t, err, "L403")

	_, err = Set("test.lkal", root, "a/child", model.Int(1))
	checkCode(t, err, "L402")

	_, err = Set("test.lkal", root, "a/b/c", model.Int(1))
	checkCode(t, err, "L402")

	_, err = Set("test.lkal", root, "svc/missing/port", model.Int(1))
	checkCode(t, err, "L402")

	_, err = Delete("test.lkal", root, "bad/")
	checkCode(t, err, "L401")

	_, err = Delete("test.lkal", root, "missing")
	checkCode(t, err, "L402")

	_, err = Delete("test.lkal", root, "svc/missing")
	checkCode(t, err, "L402")
}
