package schema

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Kaikei-e/LKAL"
)

// assertLinear8x checks that run4N costs at most 8x runN. The smaller batch is
// repeated until it takes at least 25ms, and the best of five paired ratios is
// kept. A quadratic implementation is about 16x, so this still fails a real
// super-linear regression. Millisecond-scale samples under the race detector
// no longer fail the check by themselves.
func assertLinear8x(t *testing.T, runN, run4N func()) {
	t.Helper()
	reps := 1
	for reps < 4096 {
		start := time.Now()
		for i := 0; i < reps; i++ {
			runN()
		}
		if time.Since(start) >= 25*time.Millisecond {
			break
		}
		reps *= 2
	}
	best := math.MaxFloat64
	for trial := 0; trial < 5; trial++ {
		runtime.GC()
		start := time.Now()
		for i := 0; i < reps; i++ {
			runN()
		}
		dN := time.Since(start)
		runtime.GC()
		start = time.Now()
		for i := 0; i < reps; i++ {
			run4N()
		}
		d4 := time.Since(start)
		ratio := math.Inf(1)
		if dN > 0 {
			ratio = float64(d4) / float64(dN)
		}
		if ratio < best {
			best = ratio
		}
		t.Logf("reps=%d trial=%d N=%v 4N=%v ratio=%.2fx", reps, trial, dN, d4, ratio)
	}
	if best > 8 {
		t.Errorf("best 4N/N ratio %.2fx exceeds 8x", best)
	}
}

func TestValidateScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling test in short mode")
	}

	s := mustCompile(t, `@lkal "0.1"

lkal-schema = "0.2"

root {
  field "tags" {
    type = .list
    unique = true
    items { type = .string, min-length = 1, max-length = 32 }
  }
  field "service" {
    type = .map
    unique-by = ["order"]
    key { min-length = 2, max-length = 32, pattern = "[a-z][a-z0-9_-]*" }
    value {
      type = .record
      field "order" { required = true, type = .int, min = 1 }
      field "replicas" { type = .int, min = 0, max = 100, default = 1 }
      field "protocol" { type = .enum, values = [.tcp, .udp], default = .tcp }
      field "memory" { type = .quantity, units = ["Mi", "Gi"] }
      field "secret" { type = .ref, targets = [&secret] }
      field "ratio" { types = [.int, .decimal], min = 0, max = 1 }
    }
  }
}
`)

	buildDoc := func(n int, corruptLast bool) *lkal.File {
		var b strings.Builder
		b.Grow(n * 180)
		b.WriteString("@lkal \"0.1\"\n\ntags = [")
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "\"tag-%d\"", i)
		}
		b.WriteString("]\n\n")
		for i := 0; i < n; i++ {
			ratio := "0.5"
			if i%2 == 0 {
				ratio = "1"
			}
			replicas := i % 10
			if corruptLast && i == n-1 {
				replicas = 999
			}
			fmt.Fprintf(&b,
				"service \"svc-%05d\" {\n  order = %d\n  replicas = %d\n  protocol = .tcp\n  memory = 256Mi\n  secret = &secret/svc-%05d\n  ratio = %s\n}\n",
				i, i+1, replicas, i, ratio,
			)
		}
		f, err := lkal.Parse("scale.lkal", []byte(b.String()))
		if err != nil {
			t.Fatalf("Parse(n=%d) failed: %v", n, err)
		}
		return f
	}

	cases := []struct {
		name        string
		baseN       int
		corruptLast bool
	}{
		{name: "valid_rich_map_and_list", baseN: 500, corruptLast: false},
		{name: "partially_invalid_rich_map", baseN: 500, corruptLast: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docN := buildDoc(tc.baseN, tc.corruptLast)
			doc4N := buildDoc(tc.baseN*4, tc.corruptLast)

			if tc.corruptLast {
				if err := s.Validate(docN); err == nil {
					t.Fatalf("expected error on corrupt docN")
				}
				if err := s.Validate(doc4N); err == nil {
					t.Fatalf("expected error on corrupt doc4N")
				}
			} else {
				if err := s.Validate(docN); err != nil {
					t.Fatalf("unexpected error on valid docN: %v", err)
				}
				if err := s.Validate(doc4N); err != nil {
					t.Fatalf("unexpected error on valid doc4N: %v", err)
				}
			}

			assertLinear8x(t,
				func() { _ = s.Validate(docN) },
				func() { _ = s.Validate(doc4N) },
			)
		})
	}

	assertLinear := func(t *testing.T, sch *Schema, docN, doc4N *lkal.File, wantErr bool) {
		t.Helper()
		checkErr := func(label string, err error) {
			if wantErr && err == nil {
				t.Fatalf("expected error on %s", label)
			}
			if !wantErr && err != nil {
				t.Fatalf("unexpected error on %s: %v", label, err)
			}
		}
		checkErr("docN", sch.Validate(docN))
		checkErr("doc4N", sch.Validate(doc4N))
		assertLinear8x(t,
			func() { _ = sch.Validate(docN) },
			func() { _ = sch.Validate(doc4N) },
		)
	}

	t.Run("composite_unique_by", func(t *testing.T) {
		sch := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "item" {
    type = .map
    unique-by = ["coords", "meta"]
    value {
      type = .record
      field "coords" { type = .list, items { type = .int } }
      field "meta" {
        type = .record
        field "a" { type = .int }
        field "b" { type = .string }
      }
    }
  }
}
`)
		build := func(n int) *lkal.File {
			var b strings.Builder
			b.Grow(n * 80)
			b.WriteString("@lkal \"0.1\"\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&b, "item \"k%05d\" {\n  coords = [%d, %d]\n  meta { a = %d, b = \"v%d\" }\n}\n", i, i, i+1, i, i)
			}
			return mustParse(t, "comp.lkal", b.String())
		}
		assertLinear(t, sch, build(500), build(2000), false)
	})

	t.Run("many_unknown_fields", func(t *testing.T) {
		sch := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "known" { type = .int }
}
`)
		build := func(n int) *lkal.File {
			var b strings.Builder
			b.Grow(n * 24)
			b.WriteString("@lkal \"0.1\"\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&b, "unknown_%05d = %d\n", i, i)
			}
			return mustParse(t, "unk.lkal", b.String())
		}
		assertLinear(t, sch, build(500), build(2000), true)
	})

	t.Run("many_invalid_map_keys", func(t *testing.T) {
		sch := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "m" {
    type = .map
    key { pattern = "ok-[0-9]+" }
    value { type = .int }
  }
}
`)
		build := func(n int) *lkal.File {
			var b strings.Builder
			b.Grow(n * 24)
			b.WriteString("@lkal \"0.1\"\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&b, "m \"bad_%05d\" = %d\n", i, i)
			}
			return mustParse(t, "badkeys.lkal", b.String())
		}
		assertLinear(t, sch, build(500), build(2000), true)
	})

	t.Run("many_invalid_list_elements", func(t *testing.T) {
		sch := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "nums" {
    type = .list
    unique = true
    items { type = .int, min = 100 }
  }
}
`)
		build := func(n int) *lkal.File {
			var b strings.Builder
			b.Grow(n * 8)
			b.WriteString("@lkal \"0.1\"\nnums = [")
			for i := 0; i < n; i++ {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString("1")
			}
			b.WriteString("]\n")
			return mustParse(t, "badlist.lkal", b.String())
		}
		assertLinear(t, sch, build(500), build(2000), true)
	})

	t.Run("many_missing_required_fields", func(t *testing.T) {
		build := func(n int) (*Schema, *lkal.File) {
			var sb strings.Builder
			sb.Grow(n * 55)
			sb.WriteString("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot {\n  field \"target\" {\n    type = .record\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&sb, "    field \"req_%05d\" { type = .int, required = true }\n", i)
			}
			sb.WriteString("  }\n}\n")
			sch := mustCompile(t, sb.String())
			doc := mustParse(t, "missing.lkal", "@lkal \"0.1\"\ntarget {}\n")
			return sch, doc
		}
		schN, docN := build(500)
		sch4N, doc4N := build(2000)
		if err := schN.Validate(docN); err == nil {
			t.Fatal("expected missing required field errors")
		}
		if err := sch4N.Validate(doc4N); err == nil {
			t.Fatal("expected missing required field errors")
		}
		assertLinear8x(t,
			func() { _ = schN.Validate(docN) },
			func() { _ = sch4N.Validate(doc4N) },
		)
	})
}
