package path_test

import . "github.com/Kaikei-e/LKAL"

var pathTSetNormalizeCases = []pathTSetCase{
	{
		name: "new_list_new_key_is_oneline",
		src: `@lkal "0.1"

a = 1
`,
		path:  "items",
		value: List{Int(1), Int(2)},
		wantFormat: `@lkal "0.1"

a = 1
items = [1, 2]
`,
	},
	{
		name: "new_list_replacing_scalar_is_oneline",
		src: `@lkal "0.1"

items = 42
`,
		path:  "items",
		value: List{Int(1), Int(2)},
		wantFormat: `@lkal "0.1"

items = [1, 2]
`,
	},
	{
		name: "oneline_list_replaced_by_list_stays_oneline",
		src: `@lkal "0.1"

items = [1, 2]
`,
		path:  "items",
		value: List{Int(3), Int(4)},
		wantFormat: `@lkal "0.1"

items = [3, 4]
`,
	},
	{
		name: "set_inside_element_body",
		src: `@lkal "0.1"

service "web" {
  replicas = 3
}
`,
		path:  "service/web/replicas",
		value: Int(5),
		wantFormat: `@lkal "0.1"

service "web" {
  replicas = 5
}
`,
	},
	{
		name: "decimal_stored_and_printed_as_normalized",
		src: `@lkal "0.1"

rate = 1.0
`,
		path:    "rate",
		value:   Decimal("1.50"),
		wantGet: Decimal("1.5"),
		wantFormat: `@lkal "0.1"

rate = 1.5
`,
	},
	{
		name: "quantity_decimal_normalized",
		src: `@lkal "0.1"

mem = 1Gi
`,
		path:    "mem",
		value:   Quantity{Number: "1.50", Unit: "Gi"},
		wantGet: Quantity{Number: "1.5", Unit: "Gi"},
		wantFormat: `@lkal "0.1"

mem = 1.5Gi
`,
	},
	{
		name: "quantity_normalized_inside_record",
		src: `@lkal "0.1"

server {}
`,
		path: "server",
		value: Record{Fields: []Field{
			{Key: "mem", Value: Quantity{Number: "1.50", Unit: "Gi"}},
		}},
		wantGet: Quantity{Number: "1.5", Unit: "Gi"},
		wantFormat: `@lkal "0.1"

server {
  mem = 1.5Gi
}
`,
	},
	{
		name: "quantity_normalized_inside_list",
		src: `@lkal "0.1"

limits = []
`,
		path: "limits",
		value: List{
			Quantity{Number: "1.50", Unit: "Gi"},
			Quantity{Number: "2.00", Unit: "Gi"},
		},
		wantGet: List{
			Quantity{Number: "1.5", Unit: "Gi"},
			Quantity{Number: "2.0", Unit: "Gi"},
		},
		wantFormat: `@lkal "0.1"

limits = [1.5Gi, 2.0Gi]
`,
	},
	{
		name: "string_containing_lf_printed_as_raw_lines",
		src: `@lkal "0.1"

msg = "hello"
`,
		path:  "msg",
		value: String("line 1\nline 2"),
		wantFormat: `@lkal "0.1"

msg =
  \\line 1
  \\line 2
`,
	},
	{
		name: "string_with_lf_inside_list_is_quoted",
		src: `@lkal "0.1"

items = []
`,
		path:  "items",
		value: List{String("a\nb")},
		wantFormat: `@lkal "0.1"

items = ["a\nb"]
`,
	},
	{
		name: "string_with_lf_and_control_char_is_quoted",
		src: `@lkal "0.1"

msg = "old"
`,
		path:  "msg",
		value: String("a\nb\x01"),
		wantFormat: `@lkal "0.1"

msg = "a\nb\u{1}"
`,
	},
	{
		name: "key_with_space_needs_quotes",
		src: `@lkal "0.1"
`,
		path:  "\"db creds\"",
		value: Int(1),
		wantFormat: `@lkal "0.1"

"db creds" = 1
`,
	},
	{
		name: "boolean_word_key_needs_quotes",
		src: `@lkal "0.1"
`,
		path:  "\"true\"",
		value: Int(1),
		wantFormat: `@lkal "0.1"

"true" = 1
`,
	},
	{
		name: "label_starting_with_dollar",
		src: `@lkal "0.1"

service "web" {
  x = 1
}
`,
		path:    "service/\"$admin\"",
		value:   Record{Fields: []Field{{Key: "enabled", Value: Bool(true)}}},
		wantGet: Bool(true),
		wantFormat: `@lkal "0.1"

service "web" {
  x = 1
}
service "$admin" {
  enabled = true
}
`,
	},
	{
		name: "anchor_example",
		src: `@lkal "0.1"

### web
service "web" {
  replicas = 3
  port "http" { number = 8080 }
}
`,
		path: "service/web/port/https",
		value: Record{Fields: []Field{
			{Key: "number", Value: Int(8443)},
		}},
		wantFormat: `@lkal "0.1"

### web
service "web" {
  replicas = 3
  port "http" { number = 8080 }
  port "https" {
    number = 8443
  }
}
`,
	},
}
