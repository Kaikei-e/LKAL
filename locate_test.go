package lkal

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFileName(t *testing.T) {
	var nilFile *File
	if got := nilFile.Name(); got != "" {
		t.Errorf("(*File)(nil).Name() = %q, want %q", got, "")
	}

	f, err := Parse("service.lkal", []byte("@lkal \"0.1\"\nhost = \"localhost\"\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if got := f.Name(); got != "service.lkal" {
		t.Errorf("f.Name() = %q, want %q", got, "service.lkal")
	}

	jf, err := FromJSON([]byte(`{"$lkal": "0.1", "host": "localhost"}`))
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}
	if got := jf.Name(); got != "" {
		t.Errorf("FromJSON Name() = %q, want %q", got, "")
	}
}

func TestLocateNilAndRoot(t *testing.T) {
	var nilFile *File
	if loc, ok := nilFile.Locate(nil); ok || loc.Key != (Pos{}) || loc.Value != (Pos{}) || loc.Elems != nil {
		t.Errorf("nilFile.Locate(nil) = (%+v, %v), want (zero, false)", loc, ok)
	}
	if loc, ok := nilFile.Locate([]string{"a"}); ok || loc.Key != (Pos{}) {
		t.Errorf("nilFile.Locate([a]) = (%+v, %v), want (zero, false)", loc, ok)
	}

	emptyBodyFile := &File{name: "empty.lkal"}
	if loc, ok := emptyBodyFile.Locate(nil); ok || loc.Key != (Pos{}) {
		t.Errorf("emptyBodyFile.Locate(nil) = (%+v, %v), want (zero, false)", loc, ok)
	}

	src := "### header comment\n\n  @lkal \"0.1\"\na = 1\n"
	f, err := Parse("root.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	wantRoot := Location{Key: Pos{Line: 3, Col: 3}, Value: Pos{Line: 3, Col: 3}}
	if loc, ok := f.Locate(nil); !ok || loc.Key != wantRoot.Key || loc.Value != wantRoot.Value || loc.Elems != nil {
		t.Errorf("f.Locate(nil) = (%+v, %v), want (%+v, true)", loc, ok, wantRoot)
	}
	if loc, ok := f.Locate([]string{}); !ok || loc.Key != wantRoot.Key || loc.Value != wantRoot.Value || loc.Elems != nil {
		t.Errorf("f.Locate([]) = (%+v, %v), want (%+v, true)", loc, ok, wantRoot)
	}
	if f.locIndex.Load() != nil {
		t.Errorf("len(path)==0 Locate built locIndex unexpectedly")
	}
}

func TestLocateScalarsQuotedKeysAndRawStrings(t *testing.T) {
	src := `@lkal "0.1"
str = "hello"
num = 42
dec = 3.14
qty = 512Mi
flag = true
mode = .active
target = &svc/api
"quoted/key" = "val"
"日本語" = 100
script =
  \\echo one
  \\echo two
`
	f, err := Parse("scalars.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	tests := []struct {
		path    []string
		wantKey Pos
		wantVal Pos
	}{
		{[]string{"str"}, Pos{2, 1}, Pos{2, 7}},
		{[]string{"num"}, Pos{3, 1}, Pos{3, 7}},
		{[]string{"dec"}, Pos{4, 1}, Pos{4, 7}},
		{[]string{"qty"}, Pos{5, 1}, Pos{5, 7}},
		{[]string{"flag"}, Pos{6, 1}, Pos{6, 8}},
		{[]string{"mode"}, Pos{7, 1}, Pos{7, 8}},
		{[]string{"target"}, Pos{8, 1}, Pos{8, 10}},
		{[]string{"quoted/key"}, Pos{9, 1}, Pos{9, 16}},
		{[]string{"日本語"}, Pos{10, 1}, Pos{10, 9}},
		{[]string{"script"}, Pos{11, 1}, Pos{12, 3}},
	}

	for _, tt := range tests {
		pathCopy := slices.Clone(tt.path)
		loc, ok := f.Locate(tt.path)
		if !ok {
			t.Fatalf("Locate(%v) returned false, want true", tt.path)
		}
		if !slices.Equal(tt.path, pathCopy) {
			t.Errorf("Locate mutated input path slice: got %v, want %v", tt.path, pathCopy)
		}
		if loc.Key != tt.wantKey || loc.Value != tt.wantVal {
			t.Errorf("Locate(%v) = (Key=%v, Value=%v), want (Key=%v, Value=%v)",
				tt.path, loc.Key, loc.Value, tt.wantKey, tt.wantVal)
		}
		if loc.Elems != nil {
			t.Errorf("Locate(%v).Elems = %v, want nil", tt.path, loc.Elems)
		}
	}
}

func TestLocateLists(t *testing.T) {
	src := `@lkal "0.1"
empty = []
inline = ["a", "b", "c"]
multiline = [
  10,
  20,
  30,
]
`
	f, err := Parse("lists.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	t.Run("EmptyList", func(t *testing.T) {
		loc, ok := f.Locate([]string{"empty"})
		if !ok {
			t.Fatalf("Locate([empty]) = false, want true")
		}
		if loc.Key != (Pos{2, 1}) || loc.Value != (Pos{2, 9}) {
			t.Errorf("Locate([empty]) = (Key=%v, Value=%v), want (2:1, 2:9)", loc.Key, loc.Value)
		}
		if loc.Elems != nil {
			t.Errorf("Locate([empty]).Elems = %v, want nil", loc.Elems)
		}
	})

	t.Run("OneLineListAndFreshSlice", func(t *testing.T) {
		loc, ok := f.Locate([]string{"inline"})
		if !ok {
			t.Fatalf("Locate([inline]) = false, want true")
		}
		if loc.Key != (Pos{3, 1}) || loc.Value != (Pos{3, 10}) {
			t.Errorf("Locate([inline]) = (Key=%v, Value=%v), want (3:1, 3:10)", loc.Key, loc.Value)
		}
		wantElems := []Pos{{3, 11}, {3, 16}, {3, 21}}
		if !slices.Equal(loc.Elems, wantElems) {
			t.Errorf("Locate([inline]).Elems = %v, want %v", loc.Elems, wantElems)
		}
		// Mutate returned Elems and ensure subsequent Locate is unaffected.
		loc.Elems[0] = Pos{999, 999}
		loc2, ok := f.Locate([]string{"inline"})
		if !ok || !slices.Equal(loc2.Elems, wantElems) {
			t.Errorf("subsequent Locate([inline]).Elems = %v, want %v", loc2.Elems, wantElems)
		}
	})

	t.Run("MultiLineList", func(t *testing.T) {
		loc, ok := f.Locate([]string{"multiline"})
		if !ok {
			t.Fatalf("Locate([multiline]) = false, want true")
		}
		if loc.Key != (Pos{4, 1}) || loc.Value != (Pos{4, 13}) {
			t.Errorf("Locate([multiline]) = (Key=%v, Value=%v), want (4:1, 4:13)", loc.Key, loc.Value)
		}
		wantElems := []Pos{{5, 3}, {6, 3}, {7, 3}}
		if !slices.Equal(loc.Elems, wantElems) {
			t.Errorf("Locate([multiline]).Elems = %v, want %v", loc.Elems, wantElems)
		}
	})
}

func TestLocateNestedRecordsAndLabelCompression(t *testing.T) {
	src := `@lkal "0.1"
server {
  tls {
    enabled = true
  }
}
port "http" {
  listen = 80
}
host = "localhost"
port "grpc" {
  listen = 9090
}
route "GET" "/users" {
  handler = "list"
}
route "POST" "/users" {
  handler = "create"
}
item "alpha" = 10
item "beta" = [1, 2]
`
	f, err := Parse("nested.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	tests := []struct {
		name      string
		path      []string
		wantKey   Pos
		wantVal   Pos
		wantElems []Pos
	}{
		{
			name:    "outer_record",
			path:    []string{"server"},
			wantKey: Pos{2, 1},
			wantVal: Pos{2, 8},
		},
		{
			name:    "inner_record",
			path:    []string{"server", "tls"},
			wantKey: Pos{3, 3},
			wantVal: Pos{3, 7},
		},
		{
			name:    "inner_leaf",
			path:    []string{"server", "tls", "enabled"},
			wantKey: Pos{4, 5},
			wantVal: Pos{4, 15},
		},
		{
			name:    "1_label_intermediate_prefix_first_appearance",
			path:    []string{"port"},
			wantKey: Pos{7, 1},
			wantVal: Pos{7, 6},
		},
		{
			name:    "1_label_first_block",
			path:    []string{"port", "http"},
			wantKey: Pos{7, 6},
			wantVal: Pos{7, 13},
		},
		{
			name:    "1_label_first_block_leaf",
			path:    []string{"port", "http", "listen"},
			wantKey: Pos{8, 3},
			wantVal: Pos{8, 12},
		},
		{
			name:    "1_label_non_contiguous_second_block",
			path:    []string{"port", "grpc"},
			wantKey: Pos{11, 6},
			wantVal: Pos{11, 13},
		},
		{
			name:    "1_label_non_contiguous_second_block_leaf",
			path:    []string{"port", "grpc", "listen"},
			wantKey: Pos{12, 3},
			wantVal: Pos{12, 12},
		},
		{
			name:    "multi_label_intermediate_level_1",
			path:    []string{"route"},
			wantKey: Pos{14, 1},
			wantVal: Pos{14, 7},
		},
		{
			name:    "multi_label_intermediate_level_2_first",
			path:    []string{"route", "GET"},
			wantKey: Pos{14, 7},
			wantVal: Pos{14, 13},
		},
		{
			name:    "multi_label_intermediate_level_2_second",
			path:    []string{"route", "POST"},
			wantKey: Pos{17, 7},
			wantVal: Pos{17, 14},
		},
		{
			name:    "multi_label_target_block",
			path:    []string{"route", "POST", "/users"},
			wantKey: Pos{17, 14},
			wantVal: Pos{17, 23},
		},
		{
			name:    "multi_label_target_leaf",
			path:    []string{"route", "POST", "/users", "handler"},
			wantKey: Pos{18, 3},
			wantVal: Pos{18, 13},
		},
		{
			name:    "labeled_assign_intermediate",
			path:    []string{"item"},
			wantKey: Pos{20, 1},
			wantVal: Pos{20, 6},
		},
		{
			name:    "labeled_assign_scalar_leaf",
			path:    []string{"item", "alpha"},
			wantKey: Pos{20, 6},
			wantVal: Pos{20, 16},
		},
		{
			name:      "labeled_assign_list_leaf",
			path:      []string{"item", "beta"},
			wantKey:   Pos{21, 6},
			wantVal:   Pos{21, 15},
			wantElems: []Pos{{21, 16}, {21, 19}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, ok := f.Locate(tt.path)
			if !ok {
				t.Fatalf("Locate(%v) = false, want true", tt.path)
			}
			if loc.Key != tt.wantKey || loc.Value != tt.wantVal {
				t.Errorf("Locate(%v) = (Key=%v, Value=%v), want (Key=%v, Value=%v)",
					tt.path, loc.Key, loc.Value, tt.wantKey, tt.wantVal)
			}
			if !slices.Equal(loc.Elems, tt.wantElems) {
				t.Errorf("Locate(%v).Elems = %v, want %v", tt.path, loc.Elems, tt.wantElems)
			}
		})
	}
}

func TestLocateFromJSONAndSetUpdated(t *testing.T) {
	t.Run("FromJSONZeroPositions", func(t *testing.T) {
		jf, err := FromJSON([]byte(`{
			"$lkal": "0.1",
			"server": {
				"host": "localhost",
				"ports": [80, 443]
			}
		}`))
		if err != nil {
			t.Fatalf("FromJSON failed: %v", err)
		}

		if loc, ok := jf.Locate(nil); !ok || loc.Key != (Pos{}) || loc.Value != (Pos{}) {
			t.Errorf("jf.Locate(nil) = (%+v, %v), want (zero, true)", loc, ok)
		}
		if loc, ok := jf.Locate([]string{"server"}); !ok || loc.Key != (Pos{}) || loc.Value != (Pos{}) {
			t.Errorf("jf.Locate([server]) = (%+v, %v), want (zero, true)", loc, ok)
		}
		if loc, ok := jf.Locate([]string{"server", "host"}); !ok || loc.Key != (Pos{}) || loc.Value != (Pos{}) {
			t.Errorf("jf.Locate([server host]) = (%+v, %v), want (zero, true)", loc, ok)
		}
		loc, ok := jf.Locate([]string{"server", "ports"})
		if !ok || loc.Key != (Pos{}) || loc.Value != (Pos{}) || len(loc.Elems) != 2 || loc.Elems[0] != (Pos{}) || loc.Elems[1] != (Pos{}) {
			t.Errorf("jf.Locate([server ports]) = (%+v, %v), want 2 zero Elems and true", loc, ok)
		}
		if _, ok := jf.Locate([]string{"server", "missing"}); ok {
			t.Errorf("jf.Locate([server missing]) = true, want false")
		}
	})

	t.Run("SetUpdatedFile", func(t *testing.T) {
		f, err := Parse("set.lkal", []byte("@lkal \"0.1\"\na = 1\n"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if err := f.Set("b/c", List{String("x"), String("y")}); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		// Original untouched entry still retains its parsed positions.
		if loc, ok := f.Locate([]string{"a"}); !ok || loc.Key != (Pos{2, 1}) || loc.Value != (Pos{2, 5}) {
			t.Errorf("f.Locate([a]) = (%+v, %v), want (2:1, 2:5, true)", loc, ok)
		}
		// Newly Set compressed entry is found with zero Pos.
		if loc, ok := f.Locate([]string{"b"}); !ok || loc.Key != (Pos{}) || loc.Value != (Pos{}) {
			t.Errorf("f.Locate([b]) = (%+v, %v), want (zero, true)", loc, ok)
		}
		if loc, ok := f.Locate([]string{"b", "c"}); !ok || len(loc.Elems) != 2 {
			t.Errorf("f.Locate([b c]) = (%+v, %v), want 2 Elems and true", loc, ok)
		}
	})
}

func TestLocateNonExistentAndScalarDescent(t *testing.T) {
	src := `@lkal "0.1"
scalar = "val"
arr = [1, 2, 3]
rec {
  inner = 42
}
port "http" {
  listen = 80
}
item "lbl" = "x"
`
	f, err := Parse("notfound.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	missingPaths := [][]string{
		{"unknown"},
		{"scalar", "child"},
		{"arr", "0"},
		{"rec", "missing"},
		{"rec", "inner", "deep"},
		{"port", "unknown"},
		{"port", "http", "missing"},
		{"port", "http", "listen", "deep"},
		{"item", "lbl", "deep"},
		{"item", "other"},
	}

	for _, p := range missingPaths {
		if loc, ok := f.Locate(p); ok || loc.Key != (Pos{}) || loc.Value != (Pos{}) || loc.Elems != nil {
			t.Errorf("Locate(%v) = (%+v, %v), want (zero, false)", p, loc, ok)
		}
	}
}

func TestLocateConcurrent(t *testing.T) {
	src := `@lkal "0.1"
scalar = "val"
nums = [10, 20, 30]
server {
  host = "localhost"
}
port "http" {
  listen = 80
}
port "grpc" {
  listen = 9090
}
route "GET" "/users" {
  handler = "list"
}
`
	f, err := Parse("concurrent.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	type query struct {
		path      []string
		wantOK    bool
		wantKey   Pos
		wantVal   Pos
		wantElems []Pos
	}
	queries := []query{
		{path: nil, wantOK: true, wantKey: Pos{1, 1}, wantVal: Pos{1, 1}},
		{path: []string{"scalar"}, wantOK: true, wantKey: Pos{2, 1}, wantVal: Pos{2, 10}},
		{path: []string{"nums"}, wantOK: true, wantKey: Pos{3, 1}, wantVal: Pos{3, 8}, wantElems: []Pos{{3, 9}, {3, 13}, {3, 17}}},
		{path: []string{"server", "host"}, wantOK: true, wantKey: Pos{5, 3}, wantVal: Pos{5, 10}},
		{path: []string{"port"}, wantOK: true, wantKey: Pos{7, 1}, wantVal: Pos{7, 6}},
		{path: []string{"port", "grpc", "listen"}, wantOK: true, wantKey: Pos{11, 3}, wantVal: Pos{11, 12}},
		{path: []string{"route", "GET"}, wantOK: true, wantKey: Pos{13, 7}, wantVal: Pos{13, 13}},
		{path: []string{"route", "GET", "/users", "handler"}, wantOK: true, wantKey: Pos{14, 3}, wantVal: Pos{14, 13}},
		{path: []string{"missing"}, wantOK: false},
		{path: []string{"scalar", "deep"}, wantOK: false},
	}

	const workers = 32
	const rounds = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for g := 0; g < workers; g++ {
		go func(id int) {
			defer wg.Done()
			<-start
			for r := 0; r < rounds; r++ {
				for _, q := range queries {
					loc, ok := f.Locate(q.path)
					if ok != q.wantOK {
						t.Errorf("worker %d Locate(%v) ok=%v, want %v", id, q.path, ok, q.wantOK)
						return
					}
					if !ok {
						continue
					}
					if loc.Key != q.wantKey || loc.Value != q.wantVal || !slices.Equal(loc.Elems, q.wantElems) {
						t.Errorf("worker %d Locate(%v) = %+v, want (Key=%v, Val=%v, Elems=%v)",
							id, q.path, loc, q.wantKey, q.wantVal, q.wantElems)
						return
					}
					// Mutate returned Elems to ensure each caller received an isolated copy.
					if len(loc.Elems) > 0 {
						loc.Elems[0] = Pos{Line: id + 100, Col: r + 100}
					}
				}
			}
		}(g)
	}
	close(start)
	wg.Wait()

	if f.locIndex.Load() == nil {
		t.Errorf("expected locIndex to be populated after concurrent Locate calls")
	}
}

func TestLocateCacheInvalidationSetDelete(t *testing.T) {
	src := `@lkal "0.1"
a = 1
port "http" {
  listen = 80
}
port "grpc" {
  listen = 9090
}
`
	f, err := Parse("invalidate.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Build cache via initial Locate.
	if loc, ok := f.Locate([]string{"a"}); !ok || loc.Key != (Pos{2, 1}) || loc.Value != (Pos{2, 5}) {
		t.Fatalf("initial Locate([a]) = (%+v, %v), want (2:1, 2:5, true)", loc, ok)
	}
	if loc, ok := f.Locate([]string{"port"}); !ok || loc.Key != (Pos{3, 1}) || loc.Value != (Pos{3, 6}) {
		t.Fatalf("initial Locate([port]) = (%+v, %v), want (3:1, 3:6, true)", loc, ok)
	}
	cached := f.locIndex.Load()
	if cached == nil {
		t.Fatalf("expected locIndex to be populated")
	}

	// Failed Set and Delete must not corrupt cached results.
	if err := f.Set("a/child", Int(2)); err == nil {
		t.Fatalf("expected error setting child of scalar")
	}
	if err := f.Delete("nonexistent"); err == nil {
		t.Fatalf("expected error deleting nonexistent path")
	}
	if f.locIndex.Load() != cached {
		t.Errorf("failed Set/Delete unexpectedly invalidated locIndex")
	}

	// Set updates existing key -> invalidates cache and reflects updated list value.
	if err := f.Set("a", List{Int(10), Int(20)}); err != nil {
		t.Fatalf("Set(a) failed: %v", err)
	}
	if f.locIndex.Load() != nil {
		t.Errorf("Set(a) did not invalidate locIndex")
	}
	if loc, ok := f.Locate([]string{"a"}); !ok || len(loc.Elems) != 2 {
		t.Errorf("Locate([a]) after Set = (%+v, %v), want 2 Elems and true", loc, ok)
	}

	// Set inserts a new nested entry -> invalidates cache and makes it discoverable.
	if err := f.Set("port/ws/listen", Int(8080)); err != nil {
		t.Fatalf("Set(port/ws/listen) failed: %v", err)
	}
	if f.locIndex.Load() != nil {
		t.Errorf("Set(port/ws/listen) did not invalidate locIndex")
	}
	if _, ok := f.Locate([]string{"port", "ws", "listen"}); !ok {
		t.Errorf("Locate([port ws listen]) = false, want true")
	}

	// Delete first labeled entry -> invalidates cache; intermediate "port" now points to "grpc" (line 6).
	if err := f.Delete("port/http"); err != nil {
		t.Fatalf("Delete(port/http) failed: %v", err)
	}
	if f.locIndex.Load() != nil {
		t.Errorf("Delete(port/http) did not invalidate locIndex")
	}
	if _, ok := f.Locate([]string{"port", "http"}); ok {
		t.Errorf("Locate([port http]) = true after Delete, want false")
	}
	if _, ok := f.Locate([]string{"port", "http", "listen"}); ok {
		t.Errorf("Locate([port http listen]) = true after Delete, want false")
	}
	if loc, ok := f.Locate([]string{"port"}); !ok || loc.Key != (Pos{6, 1}) || loc.Value != (Pos{6, 6}) {
		t.Errorf("Locate([port]) after deleting first label = (%+v, %v), want (6:1, 6:6, true)", loc, ok)
	}
	if loc, ok := f.Locate([]string{"port", "grpc", "listen"}); !ok || loc.Key != (Pos{7, 3}) || loc.Value != (Pos{7, 12}) {
		t.Errorf("Locate([port grpc listen]) = (%+v, %v), want (7:3, 7:12, true)", loc, ok)
	}

	// Delete entire prefix-compressed group -> invalidates cache; "port" and all children vanish.
	if err := f.Delete("port"); err != nil {
		t.Fatalf("Delete(port) failed: %v", err)
	}
	if f.locIndex.Load() != nil {
		t.Errorf("Delete(port) did not invalidate locIndex")
	}
	for _, p := range [][]string{{"port"}, {"port", "grpc"}, {"port", "ws", "listen"}} {
		if _, ok := f.Locate(p); ok {
			t.Errorf("Locate(%v) = true after Delete(port), want false", p)
		}
	}

	// Set replacing a compressed prefix group with a scalar -> invalidates cache.
	if err := f.Set("grp/x", Int(1)); err != nil {
		t.Fatalf("Set(grp/x) failed: %v", err)
	}
	if err := f.Set("grp/y", Int(2)); err != nil {
		t.Fatalf("Set(grp/y) failed: %v", err)
	}
	if _, ok := f.Locate([]string{"grp", "x"}); !ok {
		t.Fatalf("Locate([grp x]) = false, want true")
	}
	if err := f.Set("grp", String("replaced")); err != nil {
		t.Fatalf("Set(grp) failed: %v", err)
	}
	if f.locIndex.Load() != nil {
		t.Errorf("Set(grp) replacing prefix group did not invalidate locIndex")
	}
	if _, ok := f.Locate([]string{"grp"}); !ok {
		t.Errorf("Locate([grp]) = false after replacement, want true")
	}
	if _, ok := f.Locate([]string{"grp", "x"}); ok {
		t.Errorf("Locate([grp x]) = true after replacement with scalar, want false")
	}
}

func TestLocateScalesLinearly(t *testing.T) {
	buildFlat := func(n int) (*File, [][]string) {
		var b strings.Builder
		b.Grow(n * 20)
		b.WriteString("@lkal \"0.1\"\n")
		paths := make([][]string, n)
		for i := 0; i < n; i++ {
			k := fmt.Sprintf("k%05d", i)
			fmt.Fprintf(&b, "%s = %d\n", k, i)
			paths[i] = []string{k}
		}
		f, err := Parse("flat_scale.lkal", []byte(b.String()))
		if err != nil {
			t.Fatalf("Parse(flat n=%d) failed: %v", n, err)
		}
		return f, paths
	}

	buildLabeled := func(n int) (*File, [][]string) {
		var b strings.Builder
		b.Grow(n * 40)
		b.WriteString("@lkal \"0.1\"\n")
		paths := make([][]string, n)
		for i := 0; i < n; i++ {
			lbl := fmt.Sprintf("e%05d", i)
			fmt.Fprintf(&b, "item %q { v = %d }\n", lbl, i)
			paths[i] = []string{"item", lbl, "v"}
		}
		f, err := Parse("labeled_scale.lkal", []byte(b.String()))
		if err != nil {
			t.Fatalf("Parse(labeled n=%d) failed: %v", n, err)
		}
		return f, paths
	}

	measure := func(f *File, paths [][]string) time.Duration {
		// Verify correctness and warm up once.
		f.locIndex.Store(nil)
		for i, p := range paths {
			loc, ok := f.Locate(p)
			if !ok || loc.Key.Line != i+2 {
				t.Fatalf("Locate(%v) = (%+v, %v), want Line=%d and true", p, loc, ok, i+2)
			}
		}

		// Measure 5 batches of cold index build + all N lookups; take minimum.
		var minDur time.Duration
		for sample := 0; sample < 5; sample++ {
			runtime.GC()
			start := time.Now()
			for rep := 0; rep < 3; rep++ {
				f.locIndex.Store(nil)
				for _, p := range paths {
					if _, ok := f.Locate(p); !ok {
						t.Fatalf("Locate(%v) unexpectedly returned false", p)
					}
				}
			}
			d := time.Since(start)
			if sample == 0 || d < minDur {
				minDur = d
			}
		}
		return minDur
	}

	cases := []struct {
		name  string
		build func(int) (*File, [][]string)
	}{
		{name: "flat_2000_siblings", build: buildFlat},
		{name: "labeled_2000_siblings", build: buildLabeled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fN, pathsN := tc.build(500)
			f4N, paths4N := tc.build(2000)

			durN := measure(fN, pathsN)
			dur4N := measure(f4N, paths4N)
			ratio := 0.0
			if durN > 0 {
				ratio = float64(dur4N) / float64(durN)
			}
			t.Logf("N=500: %v, 4N=2000: %v (ratio: %.2fx)", durN, dur4N, ratio)

			// Linear scaling is ~4x; O(N^2) would be ~16x. Allow 8x headroom for CI/race jitter.
			if dur4N > 8*durN {
				t.Errorf("4N duration %v exceeds 8x N duration %v (ratio: %.2fx)", dur4N, durN, ratio)
			}
		})
	}
}
