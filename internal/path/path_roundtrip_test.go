package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"os"
	"testing"
)

func TestPathGetSetRoundTrip(t *testing.T) {
	data, err := os.ReadFile("../../testdata/conformance/valid/spec_example.lkal")
	if err != nil {
		t.Fatalf("failed to read spec_example.lkal: %v", err)
	}
	f := pathTParse(t, "spec_example.lkal", string(data))

	paths := []string{
		"service/web/replicas",
		"service/web/traffic-share",
		"service/web/labels/\"app.kubernetes.io/name\"",
		"service/web/labels/tier",
		"service/web/labels",
		"service/web/container/app/image",
		"service/web/container/app/args",
		"service/web/container/app/memory",
		"service/web/container/app/cpu",
		"service/web/container/app/timeout",
		"service/web/container/app/port/http/number",
		"service/web/container/app/port/http/protocol",
		"service/web/container/app/port/http",
		"service/web/container/app/port/metrics/number",
		"service/web/container/app/port/metrics/protocol",
		"service/web/container/app/port/metrics",
		"service/web/container/app/port",
		"service/web/container/app/env/LOG_LEVEL",
		"service/web/container/app/env",
		"service/web/container/app/secret",
		"service/web/container/app",
		"service/web/startup",
		"service/web",
		"service",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			val, err := f.Get(p)
			if err != nil {
				t.Fatalf("Get(%q) error: %v", p, err)
			}
			if val == nil {
				t.Fatalf("Get(%q) returned nil Value", p)
			}
			formatted := FormatValue(val)
			if formatted == nil {
				t.Fatalf("FormatValue(Get(%q)) returned nil", p)
			}
			parsedVal, err := ParseValue(formatted)
			if err != nil {
				t.Fatalf("ParseValue(FormatValue(Get(%q))) error: %v", p, err)
			}
			if !Equal(parsedVal, val) {
				t.Errorf("ParseValue(FormatValue(Get(%q))) not Equal:\ngot:  %#v\nwant: %#v", p, parsedVal, val)
			}

			fCopy := pathTParse(t, "spec_example.lkal", string(data))
			if err := fCopy.Set(p, val); err != nil {
				t.Fatalf("Set(%q, Get(%q)) error: %v", p, p, err)
			}
			if !Equal(fCopy.Data(), f.Data()) {
				t.Errorf("Set(%q, Get(%q)) changed Data():\ngot:  %#v\nwant: %#v", p, p, fCopy.Data(), f.Data())
			}
		})
	}
}

func TestPathNoAliasing(t *testing.T) {
	t.Run("set_caller_mutation", func(t *testing.T) {
		tests := []struct {
			name       string
			path       string
			setup      func() (Value, func())
			wantGet    Value
			wantFormat string
		}{
			{
				name: "set_ref_mutated",
				path: "x",
				setup: func() (Value, func()) {
					r := Ref{"a"}
					return r, func() {
						r[0] = "b"
					}
				},
				wantGet:    Ref{"a"},
				wantFormat: "@lkal \"0.1\"\n\nx = &a\n",
			},
			{
				name: "set_ref_inside_list_mutated",
				path: "x",
				setup: func() (Value, func()) {
					r := Ref{"a"}
					l := List{r}
					return l, func() {
						r[0] = "b"
					}
				},
				wantGet:    List{Ref{"a"}},
				wantFormat: "@lkal \"0.1\"\n\nx = [&a]\n",
			},
			{
				name: "set_ref_inside_record_mutated",
				path: "x",
				setup: func() (Value, func()) {
					r := Ref{"a"}
					rec := Record{Fields: []Field{{Key: "k", Value: r}}}
					return rec, func() {
						r[0] = "b"
					}
				},
				wantGet:    Record{Fields: []Field{{Key: "k", Value: Ref{"a"}}}},
				wantFormat: "@lkal \"0.1\"\n\nx {\n  k = &a\n}\n",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "test.lkal", "@lkal \"0.1\"\n")
				val, mutate := tc.setup()
				if err := f.Set(tc.path, val); err != nil {
					t.Fatalf("Set(%q) unexpected error: %v", tc.path, err)
				}
				mutate()

				got, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("Get(%q) unexpected error: %v", tc.path, err)
				}
				if got == nil {
					t.Fatalf("Get(%q) returned nil Value", tc.path)
				}
				if !Equal(got, tc.wantGet) {
					t.Errorf("Get(%q) after caller mutation =\ngot:  %#v\nwant: %#v", tc.path, got, tc.wantGet)
				}
				pathTCheckOrder(t, got, tc.wantGet)

				gotFmt := string(f.Format())
				if gotFmt != tc.wantFormat {
					t.Errorf("Format() after caller mutation =\ngot:\n%s\nwant:\n%s", gotFmt, tc.wantFormat)
				}
			})
		}
	})

	t.Run("get_ref_caller_mutation", func(t *testing.T) {
		specData, err := os.ReadFile("../../testdata/conformance/valid/spec_example.lkal")
		if err != nil {
			t.Fatalf("failed to read spec_example.lkal: %v", err)
		}

		tests := []struct {
			name    string
			src     string
			path    string
			mutate  func(t *testing.T, v Value)
			wantGet Value
		}{
			{
				name: "spec_example_secret",
				src:  string(specData),
				path: "service/web/container/app/secret",
				mutate: func(t *testing.T, v Value) {
					r, ok := v.(Ref)
					if !ok {
						t.Fatalf("expected Ref, got %T", v)
					}
					if len(r) == 0 {
						t.Fatalf("expected non-empty Ref")
					}
					r[0] = "tampered"
				},
				wantGet: Ref{"secret", "db-creds"},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "spec_example.lkal", tc.src)
				origFormat := string(f.Format())

				got1, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("Get(%q) unexpected error: %v", tc.path, err)
				}
				if got1 == nil {
					t.Fatalf("Get(%q) returned nil Value", tc.path)
				}

				tc.mutate(t, got1)

				got2, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("second Get(%q) unexpected error: %v", tc.path, err)
				}
				if got2 == nil {
					t.Fatalf("second Get(%q) returned nil Value", tc.path)
				}
				if !Equal(got2, tc.wantGet) {
					t.Errorf("second Get(%q) =\ngot:  %#v\nwant: %#v", tc.path, got2, tc.wantGet)
				}
				pathTCheckOrder(t, got2, tc.wantGet)

				gotFmt := string(f.Format())
				if gotFmt != origFormat {
					t.Errorf("Format() after caller modified Get result =\ngot:\n%s\nwant:\n%s", gotFmt, origFormat)
				}
			})
		}
	})

	t.Run("get_list_caller_mutation", func(t *testing.T) {
		specData, err := os.ReadFile("../../testdata/conformance/valid/spec_example.lkal")
		if err != nil {
			t.Fatalf("failed to read spec_example.lkal: %v", err)
		}

		tests := []struct {
			name    string
			src     string
			path    string
			mutate  func(t *testing.T, v Value)
			wantGet Value
		}{
			{
				name: "spec_example_args",
				src:  string(specData),
				path: "service/web/container/app/args",
				mutate: func(t *testing.T, v Value) {
					l, ok := v.(List)
					if !ok {
						t.Fatalf("expected List, got %T", v)
					}
					if len(l) == 0 {
						t.Fatalf("expected non-empty List")
					}
					l[0] = String("--tampered")
				},
				wantGet: List{String("--port"), String("8080")},
			},
			{
				name: "list_containing_ref_element",
				src:  "@lkal \"0.1\"\n\nitems = [&a, &b]\n",
				path: "items",
				mutate: func(t *testing.T, v Value) {
					l, ok := v.(List)
					if !ok {
						t.Fatalf("expected List, got %T", v)
					}
					if len(l) == 0 {
						t.Fatalf("expected non-empty List")
					}
					r, ok := l[0].(Ref)
					if !ok {
						t.Fatalf("expected Ref element in List, got %T", l[0])
					}
					r[0] = "tampered"
				},
				wantGet: List{Ref{"a"}, Ref{"b"}},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "test.lkal", tc.src)
				origFormat := string(f.Format())
				origParsed := pathTParse(t, "", origFormat)

				got1, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("Get(%q) unexpected error: %v", tc.path, err)
				}
				if got1 == nil {
					t.Fatalf("Get(%q) returned nil Value", tc.path)
				}

				tc.mutate(t, got1)

				got2, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("second Get(%q) unexpected error: %v", tc.path, err)
				}
				if got2 == nil {
					t.Fatalf("second Get(%q) returned nil Value", tc.path)
				}
				if !Equal(got2, tc.wantGet) {
					t.Errorf("second Get(%q) =\ngot:  %#v\nwant: %#v", tc.path, got2, tc.wantGet)
				}
				pathTCheckOrder(t, got2, tc.wantGet)

				gotFmt := string(f.Format())
				if gotFmt != origFormat {
					t.Errorf("Format() after caller modified Get List =\ngot:\n%s\nwant:\n%s", gotFmt, origFormat)
				}

				pathTCheckEqualAndOrder(t, f.Data(), origParsed.Data())
			})
		}
	})
}
