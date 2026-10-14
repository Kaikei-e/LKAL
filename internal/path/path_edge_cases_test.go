package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"bytes"
	"os"
	"testing"
)

func TestSetErrorOrder(t *testing.T) {
	specData, err := os.ReadFile("../../testdata/conformance/valid/spec_example.lkal")
	if err != nil {
		t.Fatalf("failed to read spec_example.lkal: %v", err)
	}

	tests := []struct {
		name     string
		src      string
		path     string
		value    Value
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "list_decimal_invalid_before_nil",
			src:      string(specData),
			path:     "new_list",
			value:    List{Decimal("1."), nil},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a decimal such as 0.25",
		},
		{
			name:     "list_enum_invalid_before_non_scalar",
			src:      string(specData),
			path:     "new_list",
			value:    List{Enum("1x"), List{}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write an enum name that starts with a letter or _",
		},
		{
			name:     "list_nil_before_decimal_invalid",
			src:      string(specData),
			path:     "new_list",
			value:    List{nil, Decimal("1.")},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a non-nil value",
		},
		{
			name:     "collection_element_path_list_nil",
			src:      string(specData),
			path:     "service/new",
			value:    List{nil},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a record for a collection element",
		},
		{
			name:     "collection_element_path_collection_empty",
			src:      string(specData),
			path:     "service/new",
			value:    Collection{},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a record for a collection element",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "spec_example.lkal", tc.src)
			origFormat := f.Format()
			err := f.Set(tc.path, tc.value)
			if err == nil {
				t.Fatalf("Set(%q, %#v) succeeded unexpectedly, want error", tc.path, tc.value)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("Set(%q) error type =\ngot:  %T (%v)\nwant: *ErrorList", tc.path, err, err)
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("Set(%q) error count =\ngot:  %d (%v)\nwant: 1", tc.path, len(errList.Errors), errList)
			}
			e := errList.Errors[0]
			if e == nil {
				t.Fatalf("Set(%q) error is nil", tc.path)
			}
			if e.Code != tc.wantCode {
				t.Errorf("Code =\ngot:  %q\nwant: %q", e.Code, tc.wantCode)
			}
			if e.Pos != (Pos{}) {
				t.Errorf("Pos =\ngot:  %v\nwant: zero Pos", e.Pos)
			}
			if e.File != "spec_example.lkal" {
				t.Errorf("File =\ngot:  %q\nwant: %q", e.File, "spec_example.lkal")
			}
			if e.Msg != tc.wantMsg {
				t.Errorf("Msg =\ngot:  %q\nwant: %q", e.Msg, tc.wantMsg)
			}
			if e.Hint != tc.wantHint {
				t.Errorf("Hint =\ngot:  %q\nwant: %q", e.Hint, tc.wantHint)
			}
			if !bytes.Equal(f.Format(), origFormat) {
				t.Errorf("File format changed after failed Set:\ngot:\n%s\nwant:\n%s", string(f.Format()), string(origFormat))
			}
		})
	}
}

func TestSetAddToEmptyBlock(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		path       string
		value      Value
		wantFormat string
		wantGet    Value
	}{
		{
			name:       "add_to_empty_record_block",
			src:        "@lkal \"0.1\"\n\nserver {}\n",
			path:       "server/mem",
			value:      Quantity{Number: "1.5", Unit: "Gi"},
			wantFormat: "@lkal \"0.1\"\n\nserver {\n  mem = 1.5Gi\n}\n",
			wantGet:    Quantity{Number: "1.5", Unit: "Gi"},
		},
		{
			name:       "add_to_empty_element_body",
			src:        "@lkal \"0.1\"\n\nport \"http\" {}\n",
			path:       "port/http/number",
			value:      Int(80),
			wantFormat: "@lkal \"0.1\"\n\nport \"http\" {\n  number = 80\n}\n",
			wantGet:    Int(80),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", tc.src)
			err := f.Set(tc.path, tc.value)
			if err != nil {
				t.Fatalf("Set(%q, %#v) unexpected error: %v", tc.path, tc.value, err)
			}
			gotFormat := string(f.Format())
			if gotFormat != tc.wantFormat {
				t.Errorf("Format() =\ngot:\n%s\nwant:\n%s", gotFormat, tc.wantFormat)
			}
			parsed := pathTParse(t, "", tc.wantFormat)
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())

			gotVal, getErr := f.Get(tc.path)
			if getErr != nil {
				t.Fatalf("Get(%q) unexpected error: %v", tc.path, getErr)
			}
			if gotVal == nil {
				t.Fatalf("Get(%q) returned nil Value", tc.path)
			}
			if !Equal(gotVal, tc.wantGet) {
				t.Errorf("Get(%q) =\ngot:  %#v\nwant: %#v", tc.path, gotVal, tc.wantGet)
			}
		})
	}
}

func TestPathEmptySegmentMissing(t *testing.T) {
	const src = "@lkal \"0.1\"\na = 1\n"
	const missingPath = "\"\"/x"
	const wantHint = `check the path at segment ""`

	negativeTests := []struct {
		name string
		op   func(t *testing.T, f *File) error
	}{
		{
			name: "Get",
			op: func(t *testing.T, f *File) error {
				t.Helper()
				val, err := f.Get(missingPath)
				if val != nil {
					t.Errorf("Get(%q) value: got %v, want nil", missingPath, val)
				}
				return err
			},
		},
		{
			name: "Set",
			op: func(t *testing.T, f *File) error {
				t.Helper()
				return f.Set(missingPath, Int(1))
			},
		},
		{
			name: "Delete",
			op: func(t *testing.T, f *File) error {
				t.Helper()
				return f.Delete(missingPath)
			},
		},
	}

	for _, tc := range negativeTests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", src)
			origFormatBytes := f.Format()
			if origFormatBytes == nil {
				t.Fatalf("Format() returned nil, want non-nil")
			}
			origFormat := string(origFormatBytes)

			var (
				err      error
				panicked bool
			)
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
						t.Errorf("%s panicked: got %v, want no panic", tc.name, r)
					}
				}()
				err = tc.op(t, f)
			}()

			if panicked {
				return
			}
			if err == nil {
				t.Fatalf("%s error: got nil, want *ErrorList", tc.name)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("%s error: got %T (%v), want *ErrorList", tc.name, err, err)
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("%s error count: got %d, want 1", tc.name, len(errList.Errors))
			}
			e := errList.Errors[0]
			if e == nil {
				t.Fatalf("%s Error[0]: got nil, want non-nil", tc.name)
			}
			if e.Code != "L402" {
				t.Errorf("%s Code: got %q, want %q", tc.name, e.Code, "L402")
			}
			if e.File != "test.lkal" {
				t.Errorf("%s File: got %q, want %q", tc.name, e.File, "test.lkal")
			}
			if e.Pos != (Pos{}) {
				t.Errorf("%s Pos: got %v, want %v", tc.name, e.Pos, Pos{})
			}
			if e.Hint != wantHint {
				t.Errorf("%s Hint: got %q, want %q", tc.name, e.Hint, wantHint)
			}

			fmtBytes := f.Format()
			if fmtBytes == nil {
				t.Fatalf("%s Format() returned nil, want non-nil", tc.name)
			}
			gotFormat := string(fmtBytes)
			if gotFormat != origFormat {
				t.Errorf("%s Format() mismatch:\ngot:\n%s\nwant:\n%s", tc.name, gotFormat, origFormat)
			}
		})
	}

	t.Run("positive", func(t *testing.T) {
		const posSrc = "@lkal \"0.1\"\n\"\" { x = 1 }\n"
		f := pathTParse(t, "test_pos.lkal", posSrc)
		var (
			val      Value
			err      error
			panicked bool
		)
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
					t.Errorf("Get(%q) panicked: got %v, want no panic", missingPath, r)
				}
			}()
			val, err = f.Get(missingPath)
		}()
		if panicked {
			return
		}
		if err != nil {
			t.Fatalf("Get(%q) error: got %v, want nil", missingPath, err)
		}
		if val == nil {
			t.Fatalf("Get(%q) value: got nil, want %v", missingPath, Int(1))
		}
		if !Equal(val, Int(1)) {
			t.Errorf("Get(%q) value: got %#v, want %#v", missingPath, val, Int(1))
		}
	})
}
