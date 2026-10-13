package lkal

import (
	"bytes"
	"encoding/json"
	"testing"
	"unicode/utf8"
)

func FuzzParse(f *testing.F) {
	seeds := [][]byte{
		[]byte("@lkal \"0.1\"\n"),
		[]byte("@lkal \"0.1\"\napp = \"web\"\n"),
		[]byte("### Comment\n@lkal \"0.1\"\ncount = 1\n"),
		[]byte("@lkal \"0.1\"\na =\n  \\\\one\n  \\\\two\n"),
		[]byte("@lkal \"0.1\"\nb =\n  \\\\one\n  \\\\\n  \\\\two\n"),
		[]byte("@lkal \"0.1\"\nc =\n  \\\\  indented\n"),
		[]byte("@lkal \"0.1\"\nd =\n  \\\\line\n  \\\\\n"),
		[]byte("@lkal \"0.1\"\ne =\n  \\\\### memo\n"),
		[]byte("@lkal \"0.1\"\ns = \"str\"\ni = 42\nd = 0.25\nb = true\ne = .prod\nq = 250m\nr = &ref/path\n"),
		[]byte("@lkal \"0.1\"\nempty = []\nnums = [1, 2, 3]\nstrs = [\"a\", \"b\",]\n"),
		[]byte("@lkal \"0.1\"\nrec { x = 1, y = 2 }\ncoll \"elem1\" { a = \"ok\" }\ncoll \"elem2\" { a = \"yes\" }\n"),
		[]byte("@lkal \"0.1\"\nlabels { app = \"web\", tier = \"frontend\" }\n"),
		[]byte("### Header\n\n@lkal \"0.1\"\n\nx = 1\n\n### Trailing\n"),
		[]byte("@lkal \"0.1\"\nservice \"web\" {\n  replicas = 3\n  port \"http\" { number = 8080, protocol = .tcp }\n}\n"),
		[]byte(""),
		[]byte("a = 1\n"),
		[]byte("@lkal \"0.2\"\n"),
		[]byte("@lkal \"0.1\"\nrec {\n"),
		[]byte("@lkal \"0.1\"\nlst = [1, 2\n"),
		[]byte("@lkal \"0.1\"\ns = \"unclosed\n"),
		[]byte("@lkal \"0.1\"\nval = bareword\n"),
		[]byte("# bad comment\n@lkal \"0.1\"\n"),
		[]byte("@lkal \"0.1\"\na = 1 # inline\n"),
		[]byte("@lkal \"0.1\"\na = 1\na = 2\n"),
		[]byte("@lkal \"0.1\"\ns \"a\" {}\ns \"a\" {}\n"),
		[]byte("@lkal \"0.1\"\na = 1\na {}\n"),
		[]byte("@lkal \"0.1\"\ns \"a\" {}\nx = 1\ns \"b\" {}\n"),
		[]byte("@lkal \"0.1\"\nlst = [1, \"two\"]\n"),
		[]byte("@lkal \"0.1\"\n$key = 1\n"),
		[]byte("@lkal \"0.1\"\ns \"a\" = 1\n"),
		[]byte("@unknown \"0.1\"\n"),
		[]byte("@lkal \"0.1\"\nqty = 1h30m\n"),
		[]byte("@lkal \"0.1\"\nbad_num = 01\n"),
		[]byte("@lkal \"0.1\"\nesc = \"\\q\"\n"),
		[]byte("@lkal \"0.1\"\nsemi = 1;\n"),
		[]byte("@lkal \"0.1\"\nheading\n= 1\n"),
		[]byte("@lkal \"0.1\"\nraw =\n  \\\\line1\n\n  \\\\line2\n"),
		[]byte("\xff\xfe\xfd\x00\x01\x02"),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := Parse("fuzz.lkal", data)
		if err == nil {
			if file == nil {
				t.Fatalf("Parse returned nil file with nil error")
			}
			d := file.Data()
			if !Equal(d, d) {
				t.Errorf("Equal(Data(), Data()) returned false")
			}

			j, err := ToJSON(file, false)
			if err != nil {
				t.Fatalf("ToJSON(false) failed: %v", err)
			}
			if !json.Valid(j) {
				t.Errorf("ToJSON(false) produced invalid JSON: %s", j)
			}

			plain, err := ToJSON(file, true)
			if err != nil {
				t.Fatalf("ToJSON(plain) failed: %v", err)
			}
			if !json.Valid(plain) {
				t.Errorf("ToJSON(plain) produced invalid JSON: %s", plain)
			}

			g, err := FromJSON(j)
			if err != nil {
				t.Fatalf("FromJSON(ToJSON) failed: %v", err)
			}
			if g == nil {
				t.Fatalf("FromJSON returned nil File")
			}
			if !Equal(g.Data(), file.Data()) {
				t.Errorf("Equal(g.Data(), file.Data()) returned false")
			}

			j2, err := ToJSON(g, false)
			if err != nil {
				t.Fatalf("ToJSON(g, false) failed: %v", err)
			}
			if !bytes.Equal(j2, j) {
				t.Errorf("roundtrip ToJSON mismatch:\ngot:\n%s\nwant:\n%s", j2, j)
			}
			return
		}

		if file != nil {
			t.Errorf("Parse returned non-nil file on error: %v", file)
		}
		errList, ok := err.(*ErrorList)
		if !ok {
			t.Fatalf("expected *ErrorList, got %T: %v", err, err)
		}
		if len(errList.Errors) < 1 || len(errList.Errors) > 20 {
			t.Errorf("error count %d not in [1, 20]", len(errList.Errors))
		}
		maxLine := bytes.Count(data, []byte("\n")) + 1
		lines := bytes.Split(data, []byte("\n"))
		for i, e := range errList.Errors {
			if e == nil {
				t.Fatalf("nil error at index %d", i)
			}
			if e.File != "fuzz.lkal" {
				t.Errorf("error File: got %q, want %q", e.File, "fuzz.lkal")
			}
			if !parserFuzzIsKnownErrorCode(e.Code) {
				t.Errorf("unknown error code: %q", e.Code)
			}
			if e.Msg == "" {
				t.Errorf("error Msg should not be empty for code %q", e.Code)
			}
			if e.Hint == "" {
				t.Errorf("error Hint should not be empty for code %q", e.Code)
			}
			if e.Pos.Line < 1 || e.Pos.Line > maxLine {
				t.Errorf("invalid error Line %d, want in [1, %d]", e.Pos.Line, maxLine)
			}
			if e.Pos.Line >= 1 && e.Pos.Line <= len(lines) {
				maxCol := 1 + utf8.RuneCount(lines[e.Pos.Line-1])
				if e.Pos.Col < 1 || e.Pos.Col > maxCol {
					t.Errorf("invalid error Col %d on line %d, want in [1, %d]", e.Pos.Col, e.Pos.Line, maxCol)
				}
			} else if e.Pos.Col < 1 {
				t.Errorf("invalid error Col %d, want >= 1", e.Pos.Col)
			}
			if i > 0 {
				prev := errList.Errors[i-1]
				if prev != nil {
					if prev.Pos.Line > e.Pos.Line ||
						(prev.Pos.Line == e.Pos.Line && prev.Pos.Col > e.Pos.Col) ||
						(prev.Pos.Line == e.Pos.Line && prev.Pos.Col == e.Pos.Col && prev.Code > e.Code) {
						t.Errorf("errors not sorted: [%d]=%v:%s vs [%d]=%v:%s", i-1, prev.Pos, prev.Code, i, e.Pos, e.Code)
					}
				}
			}
		}
	})
}

func parserFuzzIsKnownErrorCode(code string) bool {
	switch code {
	case "L001", "L002", "L003", "L004", "L005", "L006", "L007", "L008", "L009",
		"L101", "L102", "L103", "L104", "L105", "L106",
		"L201", "L202", "L203", "L204", "L205", "L206", "L207":
		return true
	default:
		return false
	}
}
