package schema

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Kaikei-e/LKAL"
)

func mustCompile(t *testing.T, src string) *Schema {
	t.Helper()
	s, err := Compile("schema.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Compile failed unexpectedly:\n%v", err)
	}
	return s
}

func mustParse(t *testing.T, name, src string) *lkal.File {
	t.Helper()
	f, err := lkal.Parse(name, []byte(src))
	if err != nil {
		t.Fatalf("lkal.Parse(%q) failed unexpectedly:\n%v", name, err)
	}
	return f
}

func assertErrorList(t *testing.T, err error) []*lkal.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var el *lkal.ErrorList
	if !errors.As(err, &el) {
		t.Fatalf("expected *lkal.ErrorList, got %T: %v", err, err)
	}
	if len(el.Errors) == 0 {
		t.Fatalf("expected non-empty ErrorList")
	}
	return el.Errors
}

func hasErrorCode(errs []*lkal.Error, code string) bool {
	for _, e := range errs {
		if e.Code == code {
			return true
		}
	}
	return false
}

func TestMetaSchemaSelfHostingAndCanonicalFormat(t *testing.T) {
	formatted, err := lkal.Format(metaBytes)
	if err != nil {
		t.Fatalf("lkal.Format(metaBytes) failed: %v", err)
	}
	if !bytes.Equal(formatted, metaBytes) {
		t.Fatalf("meta.lkal is not in canonical format.\n--- got ---\n%s\n--- formatted ---\n%s", metaBytes, formatted)
	}

	metaFile, err := lkal.Parse("meta.lkal", metaBytes)
	if err != nil {
		t.Fatalf("lkal.Parse(meta.lkal) failed: %v", err)
	}

	if err := metaSchema.Validate(metaFile); err != nil {
		t.Fatalf("metaSchema.Validate(metaFile) failed:\n%v", err)
	}

	compiled, err := Compile("meta.lkal", metaBytes)
	if err != nil {
		t.Fatalf("Compile(meta.lkal) failed:\n%v", err)
	}
	if compiled.ID() != "lkal:meta/0.2" {
		t.Errorf("ID() = %q, want %q", compiled.ID(), "lkal:meta/0.2")
	}
	if compiled.Version() != "0.2.0" {
		t.Errorf("Version() = %q, want %q", compiled.Version(), "0.2.0")
	}
	if err := compiled.Validate(metaFile); err != nil {
		t.Fatalf("compiled meta schema failed to validate meta.lkal:\n%v", err)
	}

	// Verify meta.lkal uses type = .any ONLY on `values` and `default`.
	countAny := bytes.Count(metaBytes, []byte("type = .any"))
	// values and default, both on the shared type-spec. field-spec is a use alias.
	if countAny != 2 {
		t.Errorf("expected exactly 2 occurrences of `type = .any` in meta.lkal (values and default on type-spec), got %d", countAny)
	}
}

func TestSchemaIDVersionAndNilSafety(t *testing.T) {
	var nilSchema *Schema
	if nilSchema.ID() != "" || nilSchema.Version() != "" {
		t.Errorf("nilSchema.ID()/Version() should be empty")
	}
	if v, ok := nilSchema.Default("a"); ok || v != nil {
		t.Errorf("nilSchema.Default = (%v, %v), want (nil, false)", v, ok)
	}
	if err := nilSchema.Validate(nil); err == nil {
		t.Errorf("nilSchema.Validate(nil) = nil, want error")
	}
	if err := nilSchema.ValidateValue(lkal.Record{}); err == nil {
		t.Errorf("nilSchema.ValidateValue = nil, want error")
	}
	if _, err := nilSchema.Complete(lkal.Record{}); err == nil {
		t.Errorf("nilSchema.Complete = nil err, want error")
	}

	if _, err := CompileFile(nil); err == nil {
		t.Errorf("CompileFile(nil) = nil, want error")
	}
	if _, err := CompileFile(&lkal.File{}); err == nil {
		t.Errorf("CompileFile(&lkal.File{}) = nil, want error")
	}

	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {}
`)
	if s.ID() != "" || s.Version() != "" {
		t.Errorf("unset ID/Version = (%q, %q), want empty", s.ID(), s.Version())
	}
	if err := s.Validate(nil); err == nil {
		t.Errorf("s.Validate(nil) = nil, want error")
	}
	if err := s.Validate(&lkal.File{}); err == nil {
		t.Errorf("s.Validate(&lkal.File{}) = nil, want error")
	}

	// Parse error propagation in Compile.
	_, err := Compile("bad.lkal", []byte("not valid lkal"))
	errs := assertErrorList(t, err)
	if !hasErrorCode(errs, "L105") {
		t.Errorf("expected L105 from parse failure, got %v", errs)
	}
}

func TestCompileL607UnsupportedSchemaVersion(t *testing.T) {
	// Unknown version with future keywords must immediately return ONLY L607.
	src := `@lkal "0.1"
lkal-schema = "0.3"
future-keyword = "ignored"
root {
  unknown-root-key = 123
}
`
	_, err := Compile("future.lkal", []byte(src))
	errs := assertErrorList(t, err)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error (L607 short-circuit), got %d: %v", len(errs), errs)
	}
	e := errs[0]
	if e.Code != "L607" || e.Msg != "unsupported schema version" {
		t.Errorf("got (%s, %q), want (L607, unsupported schema version)", e.Code, e.Msg)
	}
	if e.File != "future.lkal" || e.Pos != (lkal.Pos{Line: 2, Col: 15}) {
		t.Errorf("got File=%q Pos=%v, want future.lkal 2:15", e.File, e.Pos)
	}
	if e.Path != "lkal-schema" {
		t.Errorf("got Path=%q, want lkal-schema", e.Path)
	}
	if e.Hint != `this implementation accepts only "0.2"` {
		t.Errorf("got Hint=%q, want %q", e.Hint, `this implementation accepts only "0.2"`)
	}

	// Non-string lkal-schema should be L601, not L607.
	_, err = Compile("non-str.lkal", []byte("@lkal \"0.1\"\nlkal-schema = 2\nroot {}\n"))
	errs = assertErrorList(t, err)
	if !hasErrorCode(errs, "L601") || hasErrorCode(errs, "L607") {
		t.Errorf("expected L601 for non-string lkal-schema, got %v", errs)
	}
}

func TestCompileErrorsL601ToL606(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantCode string
		wantMsg  string
		wantPath string
		wantHint string // substring match
	}{
		{
			name:     "L601_missing_lkal_schema",
			src:      "@lkal \"0.1\"\nroot {}\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "lkal-schema",
		},
		{
			name:     "L601_missing_root",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root",
		},
		{
			name:     "L601_unknown_top_level_key",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nbogus = 1\nroot {}\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "bogus",
		},
		{
			name:     "L601_root_rejects_type",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { type = .record }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/type",
		},
		{
			name:     "L601_def_rejects_required",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"a\" { type = .string, required = true }\nroot {}\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "def/a/required",
		},
		{
			name:     "L601_missing_type_types_use",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"a\" { doc = \"no type\" } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/a",
		},
		{
			name:     "L601_missing_values_for_enum",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"e\" { type = .enum } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/e",
		},
		{
			name:     "L601_missing_values_for_enum_in_union",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"e\" { types = [.string, .enum] } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/e",
		},
		{
			name:     "L601_values_non_enum_element",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"e\" { type = .enum, values = [\"a\"] } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/e/values",
		},
		{
			name:     "L601_values_not_list",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"e\" { type = .enum, values = .a } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/e/values",
		},
		{
			name:     "L601_missing_items_for_list",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"l\" { type = .list } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/l",
		},
		{
			name:     "L601_missing_value_for_map",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/m",
		},
		{
			name:     "L601_non_scalar_list_items_direct",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"l\" { type = .list, items { type = .record } } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/l/items",
		},
		{
			name:     "L601_non_scalar_list_items_any",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"l\" { type = .list, items { type = .any } } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/l/items",
		},
		{
			name:     "L601_non_scalar_list_items_via_use",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"r\" { type = .record }\nroot { field \"l\" { type = .list, items { use = &def/r } } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/l/items",
		},
		{
			name:     "L601_wrong_constraint_for_type",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"n\" { type = .int, pattern = \"[0-9]+\" } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/n/pattern",
		},
		{
			name:     "L601_int_with_decimal_min",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"n\" { type = .int, min = 1.5 } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/n/min",
		},
		{
			name:     "L601_int_with_decimal_max",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"n\" { type = .int, max = 1.5 } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/n/max",
		},
		{
			name:     "L601_int_with_string_one_of",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"n\" { type = .int, one-of = [\"a\"] } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/n/one-of",
		},
		{
			name:     "L601_decimal_with_int_one_of",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"d\" { type = .decimal, one-of = [1, 2] } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/d/one-of",
		},
		{
			name:     "L601_types_single_element",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { types = [.int] } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/x/types",
		},
		{
			name:     "L601_types_duplicate",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { types = [.int, .int] } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/x/types",
		},
		{
			name:     "L601_types_composite",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { types = [.int, .record] } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/x/types",
		},
		{
			name:     "L601_units_non_ascii_letters",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"q\" { type = .quantity, units = [\"123\"] } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/q/units",
		},
		{
			name:     "L601_use_wrong_target",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { use = &other/foo } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/x/use",
		},
		{
			name:     "L601_use_wrong_segments",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { use = &def/a/b } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/x/use",
		},
		{
			name:     "L601_bool_with_constraint",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"b\" { type = .bool, min = 0 } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/b/min",
		},
		{
			name:     "L601_any_with_constraint",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"a\" { type = .any, open = true } }\n",
			wantCode: "L601",
			wantMsg:  "invalid schema",
			wantPath: "root/field/a/open",
		},
		{
			name:     "L602_unresolved_def",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { use = &def/missing } }\n",
			wantCode: "L602",
			wantMsg:  "unresolved definition",
			wantPath: "root/field/x/use",
			wantHint: `define def "missing"`,
		},
		{
			name:     "L603_self_cycle",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"a\" { use = &def/a }\nroot {}\n",
			wantCode: "L603",
			wantMsg:  "definition cycle",
			wantPath: "def/a/use",
			wantHint: "a -> a",
		},
		{
			name:     "L603_mutual_cycle",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"a\" { use = &def/b }\ndef \"b\" { use = &def/c }\ndef \"c\" { use = &def/b }\nroot {}\n",
			wantCode: "L603",
			wantMsg:  "definition cycle",
			wantPath: "def/b/use",
			wantHint: "b -> c -> b",
		},
		{
			name:     "L604_invalid_pattern",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"s\" { type = .string, pattern = \"[unclosed\" } }\n",
			wantCode: "L604",
			wantMsg:  "invalid pattern",
			wantPath: "root/field/s/pattern",
			wantHint: "error parsing regexp",
		},
		{
			name:     "L604_invalid_map_key_pattern",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map, key { pattern = \"(bad\" }, value { type = .string } } }\n",
			wantCode: "L604",
			wantMsg:  "invalid pattern",
			wantPath: "root/field/m/key/pattern",
			wantHint: "error parsing regexp",
		},
		{
			name:     "L605_default_type_mismatch",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"n\" { type = .int, default = \"abc\" } }\n",
			wantCode: "L605",
			wantMsg:  "default does not match its type",
			wantPath: "root/field/n/default",
			wantHint: "expected int, got string",
		},
		{
			name:     "L605_default_constraint_violation_via_use",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"port\" { type = .int, min = 1, max = 65535 }\nroot { field \"p\" { use = &def/port, default = 0 } }\n",
			wantCode: "L605",
			wantMsg:  "default does not match its type",
			wantPath: "root/field/p/default",
			wantHint: "must be >= 1",
		},
		{
			name:     "L605_default_record_missing_required",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"r\" { type = .record, field \"x\" { type = .int, required = true }, default {} } }\n",
			wantCode: "L605",
			wantMsg:  "default does not match its type",
			wantPath: "root/field/r/default",
			wantHint: "add x",
		},
		{
			name:     "L606_type_and_types",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { type = .int, types = [.int, .string] } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/x/types",
		},
		{
			name:     "L606_use_with_type",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"a\" { type = .int }\nroot { field \"x\" { use = &def/a, type = .int } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/x/type",
		},
		{
			name:     "L606_use_with_constraint",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"a\" { type = .int }\nroot { field \"x\" { use = &def/a, min = 1 } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/x/min",
		},
		{
			name:     "L606_required_and_default",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { type = .int, required = true, default = 1 } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/x/default",
		},
		{
			name:     "L606_min_gt_max",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"x\" { type = .decimal, min = 10.5, max = 10 } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/x/min",
		},
		{
			name:     "L606_min_length_gt_max_length",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"s\" { type = .string, min-length = 5, max-length = 2 } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/s/min-length",
		},
		{
			name:     "L606_map_key_min_length_gt_max_length",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map, key { min-length = 5, max-length = 2 }, value { type = .string } } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/m/key/min-length",
		},
		{
			name:     "L606_min_segments_gt_max_segments",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"r\" { type = .ref, min-segments = 3, max-segments = 2 } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/r/min-segments",
		},
		{
			name:     "L606_max_segments_zero",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"r\" { type = .ref, max-segments = 0 } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/r/max-segments",
		},
		{
			name:     "L606_min_items_gt_max_items",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"l\" { type = .list, min-items = 3, max-items = 1, items { type = .int } } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/l/min-items",
		},
		{
			name:     "L606_min_entries_gt_max_entries",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map, min-entries = 3, max-entries = 1, value { type = .int } } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/m/min-entries",
		},
		{
			name:     "L606_empty_values",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"e\" { type = .enum, values = [] } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/e/values",
		},
		{
			name:     "L606_empty_one_of",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"s\" { type = .string, one-of = [] } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/s/one-of",
		},
		{
			name:     "L606_empty_units",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"q\" { type = .quantity, units = [] } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/q/units",
		},
		{
			name:     "L606_empty_targets",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"r\" { type = .ref, targets = [] } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/r/targets",
		},
		{
			name:     "L606_empty_unique_by",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map, unique-by = [], value { type = .record } } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/m/unique-by",
		},
		{
			name:     "L606_field_and_reserved_in_root",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { reserved = [\"x\"], field \"x\" { type = .string } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/reserved",
		},
		{
			name:     "L606_field_and_reserved_in_record",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"r\" { type = .record, reserved = [\"old\"], field \"old\" { type = .string } } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/r/reserved",
		},
		{
			name:     "L606_unique_by_non_record_map_value",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map, unique-by = [\"id\"], value { type = .string } } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/m/unique-by",
		},
		{
			name:     "L606_unique_by_missing_field_in_closed_record",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"item\" { type = .record, field \"name\" { type = .string } }\nroot { field \"m\" { type = .map, unique-by = [\"port\"], value { use = &def/item } } }\n",
			wantCode: "L606",
			wantMsg:  "conflicting constraints",
			wantPath: "root/field/m/unique-by",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile("schema.lkal", []byte(tc.src))
			errs := assertErrorList(t, err)
			matched := false
			for _, e := range errs {
				if e.Code == tc.wantCode && e.Msg == tc.wantMsg && e.Path == tc.wantPath {
					if tc.wantHint == "" || strings.Contains(e.Hint, tc.wantHint) {
						matched = true
						break
					}
				}
			}
			if !matched {
				t.Errorf("want error (Code=%s, Msg=%q, Path=%q, Hint contains %q), got:\n%v",
					tc.wantCode, tc.wantMsg, tc.wantPath, tc.wantHint, err)
			}
		})
	}
}

func TestRecursiveDefinitionsThroughRecordAndMapAllowed(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"

def "node" {
  type = .record
  field "label" { type = .string, required = true }
  field "next" { use = &def/node }
  field "children" {
    type = .map
    value { use = &def/node }
  }
}

def "alias1" { use = &def/alias2 }
def "alias2" { use = &def/node }

root {
  field "tree" { use = &def/alias1 }
}
`)

	validDoc := mustParse(t, "tree.lkal", `@lkal "0.1"
tree {
  label = "root"
  next {
    label = "sibling"
  }
  children "left" {
    label = "leaf"
  }
}
`)
	if err := s.Validate(validDoc); err != nil {
		t.Fatalf("expected valid recursive doc, got:\n%v", err)
	}

	invalidDoc := mustParse(t, "badtree.lkal", `@lkal "0.1"
tree {
  label = "root"
  children "left" {
    next {}
  }
}
`)
	errs := assertErrorList(t, s.Validate(invalidDoc))
	if len(errs) != 2 {
		t.Fatalf("expected 2 L502 errors, got %d:\n%v", len(errs), errs)
	}
	if errs[0].Code != "L502" || errs[0].Path != "tree/children/left/label" {
		t.Errorf("err[0] = %+v", errs[0])
	}
	if errs[1].Code != "L502" || errs[1].Path != "tree/children/left/next/label" {
		t.Errorf("err[1] = %+v", errs[1])
	}
}

func TestValidateAllDataErrorsL501ToL510(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
id = "example:app"
version = "1.0.0"

def "member" {
  type = .record
  reserved = ["legacy"]
  field "client-url" { type = .string, required = true, pattern = "https://[a-z0-9.:]+" }
  field "port" { type = .int, min = 1, max = 65535 }
}

root {
  reserved = ["retired"]
  field "name" {
    type = .string
    required = true
    min-length = 2
    max-length = 5
    pattern = "[a-z]+"
    deprecated = "use display-name"
    ext {
      owner = "platform"
    }
  }
  field "mode" {
    type = .enum
    values = [.fast, .safe]
  }
  field "choice" {
    type = .string
    one-of = ["alpha", "beta"]
  }
  field "ratio" {
    type = .decimal
    min = 0
    max = 1.5
    one-of = [0.25, 0.5, 1.0]
  }
  field "limit" {
    types = [.int, .decimal]
    min = 0.5
    max = 10
  }
  field "mem" {
    type = .quantity
    units = ["Mi", "Gi"]
  }
  field "secret" {
    type = .ref
    targets = [&secret, &vault/kv]
    min-segments = 2
    max-segments = 3
  }
  field "tags" {
    type = .list
    min-items = 1
    max-items = 3
    unique = true
    items {
      type = .string
      min-length = 2
    }
  }
  field "etcd" {
    type = .record
    field "member" {
      type = .map
      min-entries = 1
      max-entries = 2
      unique-by = ["client-url", "port"]
      key {
        min-length = 2
        max-length = 4
        pattern = "m[0-9]+"
      }
      value { use = &def/member }
    }
  }
  field "open-rec" {
    type = .record
    open = true
    reserved = ["banned"]
    field "known" { type = .bool }
  }
  field "empty-rec" {
    type = .record
  }
}
`)

	t.Run("ValidDocument", func(t *testing.T) {
		doc := mustParse(t, "ok.lkal", `@lkal "0.1"
name = "abc"
mode = .fast
choice = "alpha"
ratio = 0.50
limit = 1
mem = 512Mi
secret = &secret/db
tags = ["aa", "bb"]
etcd {
  member "m0" {
    client-url = "https://10.0.0.1:2379"
    port = 2379
  }
  member "m1" {
    client-url = "https://10.0.0.2:2379"
    port = 2380
  }
}
open-rec {
  known = true
  extra = 123
}
`)
		if err := s.Validate(doc); err != nil {
			t.Fatalf("expected valid document, got:\n%v", err)
		}
	})

	t.Run("L501_TypeMismatch", func(t *testing.T) {
		doc := mustParse(t, "l501.lkal", `@lkal "0.1"
name = "abc"
limit = "ten"
etcd = 42
`)
		errs := assertErrorList(t, s.Validate(doc))
		if len(errs) != 2 {
			t.Fatalf("got %d errors, want 2:\n%v", len(errs), errs)
		}
		if errs[0].Code != "L501" || errs[0].Msg != "type mismatch" ||
			errs[0].Pos != (lkal.Pos{Line: 3, Col: 9}) || errs[0].Path != "limit" ||
			errs[0].Hint != "expected int or decimal, got string" {
			t.Errorf("err[0] = %+v", errs[0])
		}
		if errs[1].Code != "L501" || errs[1].Path != "etcd" || errs[1].Hint != "expected record, got int" {
			t.Errorf("err[1] = %+v", errs[1])
		}
	})

	t.Run("L502_MissingRequiredAtRootAndNested", func(t *testing.T) {
		doc := mustParse(t, "l502.lkal", `@lkal "0.1"
etcd "member" "m0" {
  port = 2379
}
`)
		errs := assertErrorList(t, s.Validate(doc))
		if len(errs) != 2 {
			t.Fatalf("got %d errors, want 2:\n%v", len(errs), errs)
		}
		// Root missing field points at @lkal (1:1).
		if errs[0].Code != "L502" || errs[0].Msg != "missing required field" ||
			errs[0].Pos != (lkal.Pos{Line: 1, Col: 1}) || errs[0].Path != "name" ||
			errs[0].Hint != "add name" {
			t.Errorf("err[0] = %+v", errs[0])
		}
		// Nested label-compressed member "m0" points at "m0" (2:15).
		if errs[1].Code != "L502" || errs[1].Pos != (lkal.Pos{Line: 2, Col: 15}) ||
			errs[1].Path != "etcd/member/m0/client-url" || errs[1].Hint != "add client-url" {
			t.Errorf("err[1] = %+v", errs[1])
		}
	})

	t.Run("L503_UnknownReservedDidYouMeanAllowedAndEmpty", func(t *testing.T) {
		doc := mustParse(t, "l503.lkal", `@lkal "0.1"
name = "abc"
retired = 1
mod = .fast
zzzzzz = 1
open-rec {
  banned = 1
  extra = "ok"
}
empty-rec {
  anything = 1
}
`)
		errs := assertErrorList(t, s.Validate(doc))
		if len(errs) != 5 {
			t.Fatalf("got %d errors, want 5:\n%v", len(errs), errs)
		}
		if errs[0].Code != "L503" || errs[0].Path != "retired" || errs[0].Hint != `field "retired" is reserved` {
			t.Errorf("err[0] = %+v", errs[0])
		}
		if errs[1].Code != "L503" || errs[1].Path != "mod" || errs[1].Hint != `did you mean "mode"?` {
			t.Errorf("err[1] = %+v", errs[1])
		}
		if errs[2].Code != "L503" || errs[2].Path != "zzzzzz" || !strings.HasPrefix(errs[2].Hint, "allowed fields: ") {
			t.Errorf("err[2] = %+v", errs[2])
		}
		if errs[3].Code != "L503" || errs[3].Path != "open-rec/banned" || errs[3].Hint != `field "banned" is reserved` {
			t.Errorf("err[3] = %+v", errs[3])
		}
		if errs[4].Code != "L503" || errs[4].Path != "empty-rec/anything" || errs[4].Hint != "no fields are allowed" {
			t.Errorf("err[4] = %+v", errs[4])
		}
	})

	t.Run("L504_L505_L506_L507_L508", func(t *testing.T) {
		doc := mustParse(t, "scalars.lkal", `@lkal "0.1"
name = "A"
mode = .slow
choice = "gamma"
ratio = 2.0
limit = 0
mem = 512Ki
secret = &config
tags = ["x", "yy", "zz", "ww"]
`)
		errs := assertErrorList(t, s.Validate(doc))
		codes := make([]string, len(errs))
		for i, e := range errs {
			codes[i] = fmt.Sprintf("%s@%s(%s)", e.Code, e.Path, e.Hint)
		}
		wantChecks := []struct {
			code, path, hintSub string
		}{
			{"L504", "name", "length must be >= 2"},
			{"L505", "name", "must match [a-z]+"},
			{"L506", "mode", "use one of .fast, .safe"},
			{"L506", "choice", `use one of "alpha", "beta"`},
			{"L504", "ratio", "must be <= 1.5"},
			{"L506", "ratio", "use one of 0.25, 0.5, 1.0"},
			{"L504", "limit", "must be >= 0.5"},
			{"L507", "mem", "use one of Mi, Gi"},
			{"L508", "secret", "reference must start with &secret, &vault/kv"},
			{"L504", "secret", "must have at least 2 segments"},
			{"L504", "tags", "must have at most 3 items"},
			{"L504", "tags", "element 1: length must be >= 2"},
		}
		for _, w := range wantChecks {
			found := false
			for _, e := range errs {
				if e.Code == w.code && e.Path == w.path && strings.Contains(e.Hint, w.hintSub) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("missing expected error (%s, %s, %q) in:\n%s", w.code, w.path, w.hintSub, strings.Join(codes, "\n"))
			}
		}
	})

	t.Run("L509_ListUniqueAndMapUniqueBy", func(t *testing.T) {
		doc := mustParse(t, "dup.lkal", `@lkal "0.1"
name = "abc"
tags = ["aa", "bb", "aa"]
etcd {
  member "m0" {
    client-url = "https://10.0.0.1:2379"
    port = 2379
  }
  member "m1" {
    client-url = "https://10.0.0.1:2379"
    port = 2379
  }
}
`)
		errs := assertErrorList(t, s.Validate(doc))
		if len(errs) != 3 {
			t.Fatalf("got %d errors, want 3:\n%v", len(errs), errs)
		}
		// List unique duplicate: 3rd element ("aa") at line 3, col 21.
		if errs[0].Code != "L509" || errs[0].Msg != "duplicate value" ||
			errs[0].Pos != (lkal.Pos{Line: 3, Col: 21}) || errs[0].Path != "tags" ||
			!strings.Contains(errs[0].Hint, "3") {
			t.Errorf("err[0] = %+v", errs[0])
		}
		// Map unique-by client-url at line 10, col 18.
		if errs[1].Code != "L509" || errs[1].Pos != (lkal.Pos{Line: 10, Col: 18}) ||
			errs[1].Path != "etcd/member/m1/client-url" ||
			errs[1].Hint != `client-url "https://10.0.0.1:2379" is also used by member "m0"` {
			t.Errorf("err[1] = %+v", errs[1])
		}
		// Map unique-by port at line 11, col 12.
		if errs[2].Code != "L509" || errs[2].Pos != (lkal.Pos{Line: 11, Col: 12}) ||
			errs[2].Path != "etcd/member/m1/port" ||
			errs[2].Hint != `port 2379 is also used by member "m0"` {
			t.Errorf("err[2] = %+v", errs[2])
		}
	})

	t.Run("L510_InvalidMapKeyAndMinMaxEntries", func(t *testing.T) {
		doc := mustParse(t, "mapkey.lkal", `@lkal "0.1"
name = "abc"
etcd {
  member "x" {
    client-url = "https://10.0.0.1:2379"
  }
  member "m1" {
    client-url = "https://10.0.0.2:2379"
  }
  member "m2" {
    client-url = "https://10.0.0.3:2379"
  }
}
`)
		errs := assertErrorList(t, s.Validate(doc))
		// Expect L504 max-entries on etcd/member, plus L510 min-length & L510 pattern on etcd/member/x.
		foundMaxEntries := false
		foundKeyMinLen := false
		foundKeyPat := false
		for _, e := range errs {
			if e.Code == "L504" && e.Path == "etcd/member" {
				foundMaxEntries = true
			}
			if e.Code == "L510" && e.Msg == "invalid map key" && e.Path == "etcd/member/x" && e.Pos == (lkal.Pos{Line: 4, Col: 10}) {
				if strings.Contains(e.Hint, "length") {
					foundKeyMinLen = true
				}
				if strings.Contains(e.Hint, "match") {
					foundKeyPat = true
				}
			}
		}
		if !foundMaxEntries || !foundKeyMinLen || !foundKeyPat {
			t.Errorf("missing expected errors (maxEntries=%v, keyMinLen=%v, keyPat=%v):\n%v",
				foundMaxEntries, foundKeyMinLen, foundKeyPat, errs)
		}
	})
}

func TestExactBase10ComparisonAndUnicodeRunes(t *testing.T) {
	// 50-digit numbers that would lose precision if converted to float64!
	bigLow := "10000000000000000000000000000000000000000000000000.00000000000000000001"
	bigMid := "10000000000000000000000000000000000000000000000000.00000000000000000002"
	bigHigh := "10000000000000000000000000000000000000000000000000.00000000000000000003"

	if compareBase10("-0", "0.0") != 0 {
		t.Errorf("compareBase10(-0, 0.0) != 0")
	}
	if compareBase10("-0.000", "0") != 0 {
		t.Errorf("compareBase10(-0.000, 0) != 0")
	}
	if compareBase10("-0.1", "-0.09") >= 0 {
		t.Errorf("expected -0.1 < -0.09")
	}
	if compareBase10(bigLow, bigMid) >= 0 || compareBase10(bigMid, bigHigh) >= 0 {
		t.Errorf("high-precision decimal comparison failed")
	}
	if compareBase10("-9223372036854775808", "-9223372036854775807.9") >= 0 {
		t.Errorf("expected MinInt64 < -9223372036854775807.9")
	}

	s := mustCompile(t, fmt.Sprintf(`@lkal "0.1"
lkal-schema = "0.2"
root {
  field "prec" {
    type = .decimal
    min = %s
    max = %s
  }
  field "zero" {
    types = [.int, .decimal]
    min = 0
    max = 0.0
  }
  field "word" {
    type = .string
    min-length = 3
    max-length = 3
    pattern = "日本."
  }
  field "日本語" {
    type = .int
  }
}
`, bigMid, bigHigh))

	// Valid: exact boundary, -0.0 == 0, 3-rune Japanese string (9 bytes).
	okDoc := mustParse(t, "ok.lkal", fmt.Sprintf(`@lkal "0.1"
prec = %s
zero = -0.0
word = "日本語"
`, bigMid))
	if err := s.Validate(okDoc); err != nil {
		t.Fatalf("expected valid doc, got:\n%v", err)
	}

	// Invalid: bigLow < bigMid by 1e-20, 4-rune string, Unicode typo (1 rune distance).
	badDoc := mustParse(t, "bad.lkal", fmt.Sprintf(`@lkal "0.1"
prec = %s
word = "日本語\n"
"日本話" = 1
`, bigLow))
	errs := assertErrorList(t, s.Validate(badDoc))
	if len(errs) != 4 {
		t.Fatalf("got %d errors, want 4:\n%v", len(errs), errs)
	}
	// prec out of range (L504)
	if errs[0].Code != "L504" || errs[0].Path != "prec" {
		t.Errorf("err[0] = %+v", errs[0])
	}
	// word: length 4 > 3 (L504) AND full match rejects trailing \n (L505)
	if errs[1].Code != "L504" || errs[2].Code != "L505" {
		t.Errorf("err[1..2] = %+v, %+v", errs[1], errs[2])
	}
	// Unicode Levenshtein distance 1 -> did you mean "日本語"?
	if errs[3].Code != "L503" || errs[3].Hint != `did you mean "日本語"?` {
		t.Errorf("err[3] = %+v", errs[3])
	}
}

func TestValidateValueAndL403StructuralChecks(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "n" { type = .int, required = true, min = 1 }
  field "d" { type = .decimal, one-of = [1.5, 2.0] }
  field "tags" { type = .list, unique = true, items { type = .decimal } }
}
`)

	// Valid unnormalized Decimal in ValidateValue should normalize & pass.
	okVal := lkal.Record{Fields: []lkal.Field{
		{Key: "n", Value: lkal.Int(5)},
		{Key: "d", Value: lkal.Decimal("1.50")},
	}}
	if err := s.ValidateValue(okVal); err != nil {
		t.Fatalf("ValidateValue failed unexpectedly:\n%v", err)
	}

	// Duplicate unnormalized decimals in unique list should be caught as L509.
	dupVal := lkal.Record{Fields: []lkal.Field{
		{Key: "n", Value: lkal.Int(1)},
		{Key: "tags", Value: lkal.List{lkal.Decimal("1.0"), lkal.Decimal("1.00")}},
	}}
	errs := assertErrorList(t, s.ValidateValue(dupVal))
	if len(errs) != 1 || errs[0].Code != "L509" || errs[0].File != "" || errs[0].Pos != (lkal.Pos{}) || errs[0].Path != "tags" {
		t.Errorf("unexpected error for duplicate decimal: %+v", errs)
	}

	// Non-record structurally valid value -> L501 at root path "".
	errs = assertErrorList(t, s.ValidateValue(lkal.String("hello")))
	if len(errs) != 1 || errs[0].Code != "L501" || errs[0].Path != "" || errs[0].Hint != "expected record, got string" {
		t.Errorf("unexpected error for non-record ValidateValue: %+v", errs)
	}

	// Build a record nested 1001 levels deep (depth 0..1001 -> exceeds maxBlockDepth 1000).
	deep := lkal.Value(lkal.Record{})
	for i := 0; i < 1001; i++ {
		deep = lkal.Record{Fields: []lkal.Field{{Key: "a", Value: deep}}}
	}

	// Structural L403 cases.
	badValues := []struct {
		name string
		val  lkal.Value
	}{
		{"nil", nil},
		{"invalid_utf8_string", lkal.String("\xff")},
		{"decimal_no_frac", lkal.Decimal("42")},
		{"decimal_over_100_digits", lkal.Decimal("1." + strings.Repeat("0", 100))},
		{"quantity_no_unit", lkal.Quantity{Number: "10", Unit: ""}},
		{"quantity_radix_0x", lkal.Quantity{Number: "0", Unit: "xFF"}},
		{"quantity_neg_radix_0b", lkal.Quantity{Number: "-0", Unit: "b"}},
		{"quantity_digit_in_unit", lkal.Quantity{Number: "10", Unit: "m2"}},
		{"quantity_over_100_digits", lkal.Quantity{Number: strings.Repeat("9", 101), Unit: "s"}},
		{"invalid_enum", lkal.Enum("1bad")},
		{"empty_ref", lkal.Ref{}},
		{"invalid_utf8_ref", lkal.Ref{"\xff"}},
		{"list_with_record", lkal.List{lkal.Record{}}},
		{"list_with_list", lkal.List{lkal.List{}}},
		{"list_mixed_types", lkal.List{lkal.Int(1), lkal.String("a")}},
		{"record_dollar_key", lkal.Record{Fields: []lkal.Field{{Key: "$bad", Value: lkal.Int(1)}}}},
		{"record_dup_key", lkal.Record{Fields: []lkal.Field{{Key: "a", Value: lkal.Int(1)}, {Key: "a", Value: lkal.Int(2)}}}},
		{"record_invalid_utf8_key", lkal.Record{Fields: []lkal.Field{{Key: "\xff", Value: lkal.Int(1)}}}},
		{"record_over_1000_depth", deep},
	}

	for _, tc := range badValues {
		t.Run("L403_"+tc.name, func(t *testing.T) {
			errs := assertErrorList(t, s.ValidateValue(tc.val))
			if len(errs) != 1 || errs[0].Code != "L403" || errs[0].Msg != "invalid value" || errs[0].Hint == "" {
				t.Errorf("got %+v, want single L403 with hint", errs)
			}
		})
	}
}

func TestErrorSortingAnd20ErrorCap(t *testing.T) {
	// Schema with 25 required fields.
	var sb strings.Builder
	sb.WriteString("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot {\n")
	for i := 25; i >= 1; i-- {
		fmt.Fprintf(&sb, "  field \"f%02d\" { type = .int, required = true }\n", i)
	}
	sb.WriteString("}\n")
	s := mustCompile(t, sb.String())

	// ValidateValue (unlocated) must sort by Path lexicographically and cap at 20.
	errs := assertErrorList(t, s.ValidateValue(lkal.Record{}))
	if len(errs) != 20 {
		t.Fatalf("got %d errors, want 20", len(errs))
	}
	for i := 0; i < 20; i++ {
		wantPath := fmt.Sprintf("f%02d", i+1)
		if errs[i].Path != wantPath {
			t.Errorf("errs[%d].Path = %q, want %q", i, errs[i].Path, wantPath)
		}
	}

	// FromJSON file validated via Validate also produces unlocated errors sorted by Path.
	jf, err := lkal.FromJSON([]byte(`{"$lkal": "0.1"}`))
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}
	jErrs := assertErrorList(t, s.Validate(jf))
	if len(jErrs) != 20 || jErrs[0].Path != "f01" || jErrs[19].Path != "f20" {
		t.Errorf("unexpected FromJSON errors: len=%d first=%+v last=%+v", len(jErrs), jErrs[0], jErrs[len(jErrs)-1])
	}

	// Located errors sort before unlocated errors (test sortErrors directly).
	mixed := []*lkal.Error{
		newError("", lkal.Pos{}, "b", "L501", ""),
		newError("a.lkal", lkal.Pos{Line: 5, Col: 1}, "z", "L501", ""),
		newError("", lkal.Pos{}, "a", "L502", ""),
		newError("a.lkal", lkal.Pos{Line: 2, Col: 4}, "c", "L503", ""),
		newError("a.lkal", lkal.Pos{Line: 2, Col: 4}, "b", "L501", ""),
	}
	sorted := sortErrors(mixed)
	wantOrder := []string{"2:4:L501:b", "2:4:L503:c", "5:1:L501:z", "0:0:L502:a", "0:0:L501:b"}
	for i, w := range wantOrder {
		got := fmt.Sprintf("%d:%d:%s:%s", sorted[i].Pos.Line, sorted[i].Pos.Col, sorted[i].Code, sorted[i].Path)
		if got != w {
			t.Errorf("sorted[%d] = %q, want %q", i, got, w)
		}
	}
}

func TestDefaultAndComplete(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"

def "tls" {
  type = .record
  field "enabled" { type = .bool, default = true }
  field "min-version" { type = .string, default = "1.3" }
}

def "service" {
  type = .record
  field "port" { type = .int, default = 8080 }
  field "tags" {
    type = .list
    items { type = .string }
    default = ["default-tag"]
  }
  field "tls" {
    use = &def/tls
  }
  field "retry" {
    type = .record
    default {
      attempts = 3
    }
    field "attempts" { type = .int, default = 1 }
    field "backoff" { type = .string, default = "1s" }
  }
}

root {
  field "env" { type = .string, default = "prod" }
  field "service" {
    type = .map
    key {
      pattern = "[a-z][a-z0-9-]*"
    }
    value { use = &def/service }
  }
  field "special/key" {
    type = .record
    field "opt" { type = .int, default = 7 }
  }
  field "no-default" { type = .string }
}
`)

	// Default() valid lookups.
	v, ok := s.Default("env")
	if !ok || !lkal.Equal(v, lkal.String("prod")) {
		t.Errorf("Default(env) = (%v, %v), want (prod, true)", v, ok)
	}
	v, ok = s.Default("service/web/port")
	if !ok || !lkal.Equal(v, lkal.Int(8080)) {
		t.Errorf("Default(service/web/port) = (%v, %v), want (8080, true)", v, ok)
	}
	v, ok = s.Default("service/web/tls/min-version")
	if !ok || !lkal.Equal(v, lkal.String("1.3")) {
		t.Errorf("Default(service/web/tls/min-version) = (%v, %v), want (1.3, true)", v, ok)
	}
	v, ok = s.Default(`"special/key"/opt`)
	if !ok || !lkal.Equal(v, lkal.Int(7)) {
		t.Errorf("Default(quoted seg) = (%v, %v), want (7, true)", v, ok)
	}

	// Default() deep-clone check: mutating returned slice must not mutate schema default.
	tagsVal, ok := s.Default("service/web/tags")
	if !ok {
		t.Fatalf("Default(service/web/tags) failed")
	}
	tagsVal.(lkal.List)[0] = lkal.String("mutated")
	tagsVal2, ok := s.Default("service/web/tags")
	if !ok || !lkal.Equal(tagsVal2, lkal.List{lkal.String("default-tag")}) {
		t.Errorf("Default() leaked mutable slice: got %v", tagsVal2)
	}

	// Default() negative / malformed paths.
	badPaths := []string{
		"",
		"/",
		"/env",
		"env/",
		"service//port",
		" env",
		"env ",
		"service/web /port",
		`"unclosed`,
		`"bad\x"`,
		`"surrogate\u{d800}"`,
		`"a"b`,
		"service",              // map itself has no default
		"service/web",          // map entry is not a record field
		"service/BAD_KEY/port", // violates map.key pattern
		"no-default",           // field has no default
		"nonexistent",
		"env/child", // stepping past scalar
	}
	for _, p := range badPaths {
		if got, ok := s.Default(p); ok || got != nil {
			t.Errorf("Default(%q) = (%v, %v), want (nil, false)", p, got, ok)
		}
	}

	// Complete():
	// - Populates missing root default `env = "prod"`
	// - Inside existing `service/web`: fills `port`, `tags`, AND `retry` (recursively completed with `backoff = "1s"`!)
	// - Does NOT implicitly create missing `service/web/tls` (since `tls` has no default on itself!)
	// - Does NOT implicitly create missing `"special/key"` (since it has no default on itself!)
	// - Preserves original input immutably and normalizes numbers.
	orig := lkal.Record{
		Fields: []lkal.Field{
			{
				Key: "service",
				Value: lkal.Record{
					Fields: []lkal.Field{
						{
							Key:   "web",
							Value: lkal.Record{},
						},
						{
							Key: "api",
							Value: lkal.Record{
								Fields: []lkal.Field{
									{Key: "port", Value: lkal.Int(9000)},
									{Key: "tls", Value: lkal.Record{}},
								},
							},
						},
					},
				},
			},
		},
	}

	completed, err := s.Complete(orig)
	if err != nil {
		t.Fatalf("Complete failed:\n%v", err)
	}

	// Original must remain untouched.
	origWeb := orig.Fields[0].Value.(lkal.Record).Fields[0].Value.(lkal.Record)
	if len(origWeb.Fields) != 0 {
		t.Errorf("Complete mutated input orig: %+v", orig)
	}

	want := lkal.Record{
		Fields: []lkal.Field{
			{
				Key: "service",
				Value: lkal.Record{
					Fields: []lkal.Field{
						{
							Key: "web",
							Value: lkal.Record{
								Fields: []lkal.Field{
									{Key: "port", Value: lkal.Int(8080)},
									{Key: "tags", Value: lkal.List{lkal.String("default-tag")}},
									{
										Key: "retry",
										Value: lkal.Record{
											Fields: []lkal.Field{
												{Key: "attempts", Value: lkal.Int(3)},
												{Key: "backoff", Value: lkal.String("1s")},
											},
										},
									},
								},
							},
						},
						{
							Key: "api",
							Value: lkal.Record{
								Fields: []lkal.Field{
									{Key: "port", Value: lkal.Int(9000)},
									{
										Key: "tls",
										Value: lkal.Record{
											Fields: []lkal.Field{
												{Key: "enabled", Value: lkal.Bool(true)},
												{Key: "min-version", Value: lkal.String("1.3")},
											},
										},
									},
									{Key: "tags", Value: lkal.List{lkal.String("default-tag")}},
									{
										Key: "retry",
										Value: lkal.Record{
											Fields: []lkal.Field{
												{Key: "attempts", Value: lkal.Int(3)},
												{Key: "backoff", Value: lkal.String("1s")},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			{Key: "env", Value: lkal.String("prod")},
		},
	}

	if !lkal.Equal(completed, want) {
		t.Errorf("Complete mismatch:\ngot:  %s\nwant: %s", lkal.FormatValue(completed), lkal.FormatValue(want))
	}

	// Verify exact field order (appended in schema field order).
	compWeb := completed.(lkal.Record).Fields[0].Value.(lkal.Record).Fields[0].Value.(lkal.Record)
	gotKeys := make([]string, len(compWeb.Fields))
	for i, f := range compWeb.Fields {
		gotKeys[i] = f.Key
	}
	if !slices.Equal(gotKeys, []string{"port", "tags", "retry"}) {
		t.Errorf("web field order = %v, want [port tags retry]", gotKeys)
	}

	// Idempotency: Complete(Complete(v)) == Complete(v).
	completed2, err := s.Complete(completed)
	if err != nil {
		t.Fatalf("second Complete failed: %v", err)
	}
	if !lkal.Equal(completed, completed2) {
		t.Errorf("Complete is not idempotent")
	}

	// Complete validates completed result against schema cleanly.
	if err := s.ValidateValue(completed); err != nil {
		t.Errorf("ValidateValue(completed) failed: %v", err)
	}

	// Complete rejects non-record and structurally invalid values with L403.
	for _, bad := range []lkal.Value{nil, lkal.Int(1), lkal.String("x"), lkal.List{lkal.Int(1)}, lkal.Record{Fields: []lkal.Field{{Key: "$k", Value: lkal.Int(1)}}}} {
		_, err := s.Complete(bad)
		errs := assertErrorList(t, err)
		if errs[0].Code != "L403" {
			t.Errorf("Complete(%T) got code %s, want L403", bad, errs[0].Code)
		}
	}

	// A default that expands back into the same record is rejected at compile time.
	_, err = Compile("inf.lkal", []byte(`@lkal "0.1"
lkal-schema = "0.2"
def "loop" {
  type = .record
  field "child" {
    use = &def/loop
    default {}
  }
}
root {
  field "head" {
    use = &def/loop
    default {}
  }
}
`))
	errs := assertErrorList(t, err)
	if !hasErrorCode(errs, "L606") {
		t.Fatalf("cyclic default expansion got:\n%v\nwant L606", err)
	}
	foundCycle := false
	for _, e := range errs {
		if e.Code == "L606" && strings.Contains(e.Hint, "default expansion cycles") && strings.Contains(e.Path, "default") {
			foundCycle = true
		}
	}
	if !foundCycle {
		t.Errorf("cyclic default errors = %v, want L606 on a default path", errs)
	}
}

func TestUnionOneOfCoversEveryMember(t *testing.T) {
	_, err := Compile("cover.lkal", []byte(`@lkal "0.1"
lkal-schema = "0.2"
root {
  field "timeout" {
    types = [.string, .int]
    one-of = ["none", "infinite"]
    min = 1
    max = 60
  }
}
`))
	errs := assertErrorList(t, err)
	if !hasErrorCode(errs, "L606") {
		t.Fatalf("string one-of on string|int got:\n%v\nwant L606", err)
	}

	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "n" {
    types = [.int, .decimal]
    one-of = [1, 2]
  }
}
`)
	for _, src := range []string{
		"@lkal \"0.1\"\nn = 1\n",
		"@lkal \"0.1\"\nn = 2\n",
		"@lkal \"0.1\"\nn = 1.0\n",
		"@lkal \"0.1\"\nn = 2.0\n",
	} {
		if err := s.Validate(mustParse(t, "ok.lkal", src)); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", src, err)
		}
	}
	for _, src := range []string{
		"@lkal \"0.1\"\nn = 3\n",
		"@lkal \"0.1\"\nn = 1.5\n",
	} {
		err := s.Validate(mustParse(t, "bad.lkal", src))
		errs := assertErrorList(t, err)
		if len(errs) != 1 || errs[0].Code != "L506" || errs[0].Path != "n" {
			t.Errorf("Validate(%q) = %v, want one L506 on n", src, errs)
		}
	}
}

func TestCompleteDepthHintDistinguishesDefaultExpansion(t *testing.T) {
	spec := &typeSpec{
		kind:       kindRecord,
		kindMask:   kindBit(kindRecord),
		fields:     map[string]*fieldSpec{},
		fieldOrder: []string{"a"},
	}
	spec.fields["a"] = &fieldSpec{name: "a", spec: spec}

	cur := lkal.Value(lkal.Int(1))
	for i := 0; i < 1002; i++ {
		cur = lkal.Record{Fields: []lkal.Field{{Key: "a", Value: cur}}}
	}
	if _, hint := completeNode(spec, cur, 0, false); hint != "nest blocks at most 1000 levels deep" {
		t.Errorf("input nesting hint = %q", hint)
	}
	if _, hint := completeNode(spec, cur, 0, true); hint != "default expansion exceeded 1000 levels" {
		t.Errorf("default expansion hint = %q", hint)
	}
}

func TestValidateDoesNotInjectDefaults(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "a" { type = .int, default = 42 }
}
`)
	f := mustParse(t, "empty.lkal", "@lkal \"0.1\"\n")
	if err := s.Validate(f); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(f.Data().Fields) != 0 {
		t.Errorf("Validate mutated file or injected defaults: %+v", f.Data())
	}
}

func TestUniqueByOpenRecordAndListUniqueScalarTypes(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "open-map" {
    type = .map
    unique-by = ["id"]
    value {
      type = .record
      open = true
    }
  }
  field "refs" {
    type = .list
    unique = true
    items { type = .ref }
  }
  field "qtys" {
    type = .list
    unique = true
    items { type = .quantity }
  }
  field "map-one-of-key" {
    type = .map
    key { one-of = ["prod", "staging"] }
    value { type = .string }
  }
}
`)

	doc := mustParse(t, "u.lkal", `@lkal "0.1"
open-map "a" {
  id = [1, 2]
}
open-map "b" {
  other = true
}
open-map "c" {
  id = [1, 2]
}
refs = [&a/b, &a/b]
qtys = [10s, 10m, 10s]
map-one-of-key "dev" = "nope"
`)
	errs := assertErrorList(t, s.Validate(doc))
	if len(errs) != 4 {
		t.Fatalf("got %d errors, want 4:\n%v", len(errs), errs)
	}
	if errs[0].Code != "L509" || errs[0].Path != "open-map/c/id" ||
		errs[0].Hint != `id [1, 2] is also used by open-map "a"` {
		t.Errorf("err[0] = %+v", errs[0])
	}
	if errs[1].Code != "L509" || errs[1].Path != "refs" {
		t.Errorf("err[1] = %+v", errs[1])
	}
	if errs[2].Code != "L509" || errs[2].Path != "qtys" {
		t.Errorf("err[2] = %+v", errs[2])
	}
	if errs[3].Code != "L510" || errs[3].Path != "map-one-of-key/dev" {
		t.Errorf("err[3] = %+v", errs[3])
	}
}

func TestScalarUnionConstraintsAndMultiErrorCollection(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "label" {
    type = .string
    one-of = ["none", "infinite"]
    default = "none"
  }
  field "timeout" {
    types = [.string, .int]
    min = 1
    max = 60
  }
  field "quoted/path" {
    type = .record
    field "true" { type = .int, required = true }
  }
}
`)

	v, ok := s.Default("label")
	if !ok || !lkal.Equal(v, lkal.String("none")) {
		t.Errorf("Default(label) = (%v, %v), want (none, true)", v, ok)
	}

	okDoc := mustParse(t, "u_ok.lkal", `@lkal "0.1"
label = "none"
timeout = 30
"quoted/path" {
  "true" = 1
}
`)
	if err := s.Validate(okDoc); err != nil {
		t.Fatalf("expected valid doc, got:\n%v", err)
	}

	badDoc := mustParse(t, "u_bad.lkal", `@lkal "0.1"
label = "never"
"quoted/path" {}
`)
	errs := assertErrorList(t, s.Validate(badDoc))
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2:\n%v", len(errs), errs)
	}
	if errs[0].Code != "L506" || errs[0].Path != "label" {
		t.Errorf("err[0] = %+v", errs[0])
	}
	if errs[1].Code != "L502" || errs[1].Path != `"quoted/path"/true` || errs[1].Hint != `add "true"` {
		t.Errorf("err[1] = %+v", errs[1])
	}

	// Multiple compile errors collected together in one Compile call.
	multiBad := `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "a" { use = &def/nope }
  field "b" { type = .string, pattern = "(" }
  field "c" { type = .int, min = 10, max = 5 }
  field "d" { type = .int, default = "bad" }
  field "e" { type = .list, items { use = &def/chain } }
}
def "chain" { use = &def/rec }
def "rec" { type = .record }
`
	_, err := Compile("multi.lkal", []byte(multiBad))
	cErrs := assertErrorList(t, err)
	for _, wantCode := range []string{"L601", "L602", "L604", "L605", "L606"} {
		if !hasErrorCode(cErrs, wantCode) {
			t.Errorf("expected %s in multi-error compile, got:\n%v", wantCode, err)
		}
	}

	// CompileFile on FromJSON-constructed schema.
	jsonSchema := `{
		"$lkal": "0.1",
		"lkal-schema": "0.2",
		"id": "json:schema",
		"version": "1.0",
		"root": {
			"field": {
				"count": { "type": {"$enum": "int"}, "min": 1 }
			}
		}
	}`
	jf, err := lkal.FromJSON([]byte(jsonSchema))
	if err != nil {
		t.Fatalf("FromJSON schema failed: %v", err)
	}
	js, err := CompileFile(jf)
	if err != nil {
		t.Fatalf("CompileFile(FromJSON) failed: %v", err)
	}
	if js.ID() != "json:schema" || js.Version() != "1.0" {
		t.Errorf("ID/Version = (%q, %q)", js.ID(), js.Version())
	}
	if err := js.ValidateValue(lkal.Record{Fields: []lkal.Field{{Key: "count", Value: lkal.Int(0)}}}); err == nil {
		t.Errorf("expected L504 for count=0")
	}
}

func TestEmptyDefNameUseCyclesAndItems(t *testing.T) {
	// Self-cycle on def "" must return L603 (never hang).
	_, err := Compile("self_empty.lkal", []byte(`@lkal "0.1"
lkal-schema = "0.2"
def "" { use = &def/"" }
root { field "x" { use = &def/"" } }
`))
	errs := assertErrorList(t, err)
	if !hasErrorCode(errs, "L603") {
		t.Fatalf("expected L603 for def \"\" self cycle, got:\n%v", err)
	}
	if errs[0].Path != `def/""/use` {
		t.Errorf("got Path=%q, want def/\"\"/use", errs[0].Path)
	}

	// Mutual cycle involving def "" must return L603 (never hang).
	_, err = Compile("mutual_empty.lkal", []byte(`@lkal "0.1"
lkal-schema = "0.2"
def "a" { use = &def/"" }
def "" { use = &def/a }
root { field "x" { use = &def/a } }
`))
	errs = assertErrorList(t, err)
	if !hasErrorCode(errs, "L603") {
		t.Fatalf("expected L603 for mutual cycle with def \"\", got:\n%v", err)
	}

	// Non-scalar list items referencing def "" must point L601 at &def/"".
	_, err = Compile("items_empty.lkal", []byte(`@lkal "0.1"
lkal-schema = "0.2"
def "" { type = .record }
root {
  field "l" {
    type = .list
    items { use = &def/"" }
  }
}
`))
	errs = assertErrorList(t, err)
	if len(errs) != 1 || errs[0].Code != "L601" || errs[0].Pos != (lkal.Pos{Line: 7, Col: 19}) {
		t.Errorf("expected L601 at 7:19 (&def/\"\"), got: %+v", errs)
	}
}

func TestUniqueByEmptyFirstKeyAndCompositeEquality(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "scalars" {
    type = .map
    unique-by = ["id"]
    value {
      type = .record
      field "id" { type = .string }
    }
  }
  field "lists" {
    type = .map
    unique-by = ["id"]
    value {
      type = .record
      field "id" { type = .list, items { type = .int } }
    }
  }
  field "recs" {
    type = .map
    unique-by = ["id"]
    value {
      type = .record
      field "id" {
        type = .record
        field "a" { type = .int }
        field "b" { type = .string }
      }
    }
  }
}
`)

	doc := mustParse(t, "empty_key.lkal", `@lkal "0.1"
scalars "" { id = "dup" }
scalars "b" { id = "dup" }
lists "" { id = [1, 2] }
lists "b" { id = [1, 2] }
lists "c" { id = [1, 2] }
recs "" {
  id {
    a = 1
    b = "x"
  }
}
recs "b" {
  id {
    b = "x"
    a = 1
  }
}
`)
	errs := assertErrorList(t, s.Validate(doc))
	if len(errs) != 4 {
		t.Fatalf("got %d errors, want 4:\n%v", len(errs), errs)
	}
	wantHints := []string{
		`id "dup" is also used by scalars ""`,
		`id [1, 2] is also used by lists ""`,
		`id [1, 2] is also used by lists ""`,
		`id { b = "x", a = 1 } is also used by recs ""`,
	}
	for i, w := range wantHints {
		if errs[i].Code != "L509" || errs[i].Hint != w {
			t.Errorf("err[%d] = (%s, %q), want (L509, %q)", i, errs[i].Code, errs[i].Hint, w)
		}
	}

	// Verify valueEqualKey collision freedom across tricky boundary values.
	distinct := []lkal.Value{
		lkal.String("1:a"),
		lkal.String("1:a2:b"),
		lkal.Int(1),
		lkal.Decimal("1.0"),
		lkal.String("1"),
		lkal.Bool(true),
		lkal.Bool(false),
		lkal.Enum("true"),
		lkal.Quantity{Number: "1", Unit: "s"},
		lkal.Quantity{Number: "1s", Unit: ""},
		lkal.Ref{"a", "b"},
		lkal.Ref{"a/b"},
		lkal.List{lkal.String("a"), lkal.String("b")},
		lkal.List{lkal.String("ab")},
		lkal.Record{Fields: []lkal.Field{{Key: "a", Value: lkal.String("1:b")}}},
		lkal.Record{Fields: []lkal.Field{{Key: "a1", Value: lkal.String(":b")}}},
	}
	seen := make(map[string]int, len(distinct))
	for i, v := range distinct {
		k := valueEqualKey(v)
		if prev, dup := seen[k]; dup {
			t.Errorf("valueEqualKey collision between index %d (%#v) and %d (%#v): key=%q", prev, distinct[prev], i, v, k)
		}
		seen[k] = i
	}
	// Field order invariance in Record.
	r1 := lkal.Record{Fields: []lkal.Field{{Key: "b", Value: lkal.Int(2)}, {Key: "a", Value: lkal.Int(1)}}}
	r2 := lkal.Record{Fields: []lkal.Field{{Key: "a", Value: lkal.Int(1)}, {Key: "b", Value: lkal.Int(2)}}}
	if valueEqualKey(r1) != valueEqualKey(r2) {
		t.Errorf("valueEqualKey differs for equal records with different field order")
	}
}

func TestOneOfConflictsAndDuplicateUniqueBy(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantCode string
		wantPath string
	}{
		{
			name:     "L606_string_one_of_violates_min_length",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"s\" { type = .string, min-length = 5, one-of = [\"abc\", \"abcdef\"] } }\n",
			wantCode: "L606",
			wantPath: "root/field/s/one-of",
		},
		{
			name:     "L606_string_one_of_violates_max_length",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"s\" { type = .string, max-length = 2, one-of = [\"ab\", \"abc\"] } }\n",
			wantCode: "L606",
			wantPath: "root/field/s/one-of",
		},
		{
			name:     "L606_string_one_of_violates_pattern",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"s\" { type = .string, pattern = \"[0-9]+\", one-of = [\"123\", \"abc\"] } }\n",
			wantCode: "L606",
			wantPath: "root/field/s/one-of",
		},
		{
			name:     "L606_int_one_of_violates_min",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"n\" { type = .int, min = 10, one-of = [5, 10] } }\n",
			wantCode: "L606",
			wantPath: "root/field/n/one-of",
		},
		{
			name:     "L606_int_one_of_violates_max",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"n\" { type = .int, max = 10, one-of = [10, 11] } }\n",
			wantCode: "L606",
			wantPath: "root/field/n/one-of",
		},
		{
			name:     "L606_decimal_one_of_violates_min",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"d\" { type = .decimal, min = 1.5, one-of = [1.25, 2.0] } }\n",
			wantCode: "L606",
			wantPath: "root/field/d/one-of",
		},
		{
			name:     "L606_decimal_one_of_violates_max",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"d\" { type = .decimal, max = 1, one-of = [0.5, 1.5] } }\n",
			wantCode: "L606",
			wantPath: "root/field/d/one-of",
		},
		{
			name:     "L606_map_key_one_of_violates_pattern_and_length",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map, key { min-length = 3, pattern = \"[a-z]+\", one-of = [\"ab\", \"ABC\"] }, value { type = .string } } }\n",
			wantCode: "L606",
			wantPath: "root/field/m/key/one-of",
		},
		{
			name:     "L601_duplicate_unique_by",
			src:      "@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map, unique-by = [\"id\", \"id\"], value { type = .record, field \"id\" { type = .string } } } }\n",
			wantCode: "L601",
			wantPath: "root/field/m/unique-by",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile("schema.lkal", []byte(tc.src))
			errs := assertErrorList(t, err)
			matched := false
			for _, e := range errs {
				if e.Code == tc.wantCode && e.Path == tc.wantPath {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("want (%s, %q), got:\n%v", tc.wantCode, tc.wantPath, err)
			}
		})
	}
}

func TestLevenshteinPruningAndLargeUnknownKeySpeed(t *testing.T) {
	if d := runeLevenshtein("abc", "abc"); d != 0 {
		t.Errorf("dist(abc, abc) = %d, want 0", d)
	}
	if d := runeLevenshtein("abc", "axc"); d != 1 {
		t.Errorf("dist(abc, axc) = %d, want 1", d)
	}
	if d := runeLevenshtein("abc", "axcy"); d != 2 {
		t.Errorf("dist(abc, axcy) = %d, want 2", d)
	}
	if d := runeLevenshtein("abc", "abcdef"); d != 3 {
		t.Errorf("dist(abc, abcdef) = %d, want 3 (length pruned)", d)
	}
	if d := runeLevenshtein("aaaaaa", "bbbbbb"); d != 3 {
		t.Errorf("dist(aaaaaa, bbbbbb) = %d, want 3 (rowMin pruned)", d)
	}

	// 20,000-rune unknown key against 50 declared fields must complete almost instantaneously.
	declared := make([]string, 50)
	for i := range declared {
		declared[i] = fmt.Sprintf("field_%02d", i)
	}
	longKey := strings.Repeat("x", 20000)
	start := time.Now()
	hint := unknownFieldHint(longKey, declared)
	elapsed := time.Since(start)
	if !strings.HasPrefix(hint, "allowed fields: ") {
		t.Errorf("unexpected hint: %q", hint)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("unknownFieldHint on 20,000-rune key took %v (expected < 100ms)", elapsed)
	}

	// Same-length 20,000-rune candidate differing in the first 3 runes must also exit early.
	longCand := "yyy" + strings.Repeat("x", 19997)
	start = time.Now()
	d := runeLevenshtein(longKey, longCand)
	elapsed = time.Since(start)
	if d != 3 {
		t.Errorf("dist = %d, want 3", d)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("runeLevenshtein early-exit took %v (expected < 100ms)", elapsed)
	}
}
