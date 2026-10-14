package path_test

import . "github.com/Kaikei-e/LKAL"

import "testing"

type pathTSetCase struct {
	name       string
	src        string
	path       string
	value      Value
	wantFormat string
	wantGet    Value
}

var pathTSetLayoutCases = []pathTSetCase{
	{
		name: "replace_scalar_keeping_comments_and_blank_line",
		src: `@lkal "0.1"

first = 1

### count comment
count = 2
`,
		path:  "count",
		value: Int(10),
		wantFormat: `@lkal "0.1"

first = 1

### count comment
count = 10
`,
	},
	{
		name: "add_new_key_appended_after_last_entry_before_trailing_comments",
		src: `@lkal "0.1"

a = 1

### trailing note
`,
		path:  "b",
		value: Int(2),
		wantFormat: `@lkal "0.1"

a = 1
b = 2

### trailing note
`,
	},
	{
		name: "replace_assignment_by_record",
		src: `@lkal "0.1"

a = 1
`,
		path: "a",
		value: Record{Fields: []Field{
			{Key: "x", Value: Int(10)},
		}},
		wantFormat: `@lkal "0.1"

a {
  x = 10
}
`,
	},
	{
		name: "replace_record_by_scalar",
		src: `@lkal "0.1"

a {
  x = 10
}
`,
		path:  "a",
		value: Int(42),
		wantFormat: `@lkal "0.1"

a = 42
`,
	},
	{
		name: "multiline_list_replaced_by_list_stays_multiline",
		src: `@lkal "0.1"

items = [
  1,
  2,
]
`,
		path:  "items",
		value: List{Int(3), Int(4)},
		wantFormat: `@lkal "0.1"

items = [
  3,
  4,
]
`,
	},
	{
		name: "oneline_record_replaced_by_record_stays_oneline_when_qualifies",
		src: `@lkal "0.1"

config { x = 1, y = 2 }
`,
		path: "config",
		value: Record{Fields: []Field{
			{Key: "a", Value: Int(10)},
			{Key: "b", Value: String("hi")},
		}},
		wantFormat: `@lkal "0.1"

config { a = 10, b = "hi" }
`,
	},
	{
		name: "replace_collection_key_by_scalar",
		src: `@lkal "0.1"

item "one" {
  x = 1
}
item "two" {
  x = 2
}
trailing = true
`,
		path:  "item",
		value: String("single"),
		wantFormat: `@lkal "0.1"

item = "single"
trailing = true
`,
	},
	{
		name: "set_collection_value_consecutive_elements",
		src: `@lkal "0.1"

item = "single"
trailing = true
`,
		path: "item",
		value: Collection{Elems: []Elem{
			{Label: "first", Body: Record{Fields: []Field{{Key: "x", Value: Int(1)}}}},
			{Label: "second", Body: Record{Fields: []Field{{Key: "y", Value: Int(2)}}}},
		}},
		wantFormat: `@lkal "0.1"

item "first" {
  x = 1
}
item "second" {
  y = 2
}
trailing = true
`,
	},
	{
		name: "add_element_to_collection_followed_by_another_entry",
		src: `@lkal "0.1"

item "first" {
  x = 1
}
trailing = true
`,
		path:  "item/second",
		value: Record{Fields: []Field{{Key: "y", Value: Int(2)}}},
		wantFormat: `@lkal "0.1"

item "first" {
  x = 1
}
item "second" {
  y = 2
}
trailing = true
`,
	},
	{
		name: "replace_element_body_keeping_comments",
		src: `@lkal "0.1"

### element comment
item "first" {
  x = 1
}
`,
		path:  "item/first",
		value: Record{Fields: []Field{{Key: "x", Value: Int(99)}}},
		wantFormat: `@lkal "0.1"

### element comment
item "first" {
  x = 99
}
`,
	},
	{
		name: "replace_element_body_keeps_oneline",
		src: `@lkal "0.1"

port "http" { number = 8080 }
`,
		path: "port/http",
		value: Record{Fields: []Field{
			{Key: "number", Value: Int(9000)},
		}},
		wantFormat: `@lkal "0.1"

port "http" { number = 9000 }
`,
	},
	{
		name: "replace_record_entry_keeps_entry_comments_and_drops_body_comments",
		src: `@lkal "0.1"

first = 1

### record entry comment
server {
  ### old body comment
  old_field = 1
}
`,
		path: "server",
		value: Record{Fields: []Field{
			{Key: "new_field", Value: Int(2)},
		}},
		wantFormat: `@lkal "0.1"

first = 1

### record entry comment
server {
  new_field = 2
}
`,
	},
}

func TestSet(t *testing.T) {
	tests := append(append([]pathTSetCase(nil), pathTSetLayoutCases...), pathTSetNormalizeCases...)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", tc.src)
			err := f.Set(tc.path, tc.value)
			if err != nil {
				t.Fatalf("Set(%q, ...) unexpected error: %v", tc.path, err)
			}
			gotFormat := string(f.Format())
			if gotFormat != tc.wantFormat {
				t.Errorf("Format() mismatch:\ngot:\n%s\nwant:\n%s", gotFormat, tc.wantFormat)
			}
			parsed := pathTParse(t, "", tc.wantFormat)
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())

			if tc.wantGet != nil {
				getPath := tc.path
				if tc.name == "label_starting_with_dollar" {
					getPath = tc.path + "/enabled"
				} else if tc.name == "quantity_normalized_inside_record" {
					getPath = tc.path + "/mem"
				}
				val, getErr := f.Get(getPath)
				if getErr != nil {
					t.Fatalf("Get(%q) unexpected error: %v", getPath, getErr)
				}
				if !Equal(val, tc.wantGet) {
					t.Errorf("Get(%q) = %#v, want %#v", getPath, val, tc.wantGet)
				}
			}
		})
	}
}
