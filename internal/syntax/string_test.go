package syntax

import (
	"slices"
	"testing"
)

func TestQuoteStringAndFormatters(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hello", `"hello"`},
		{`a"b\c`, `"a\"b\\c"`},
		{"a\nb\rc\td", "\"a\\nb\\rc\td\""},
		{"\x00\x1f\x7f", `"\u{0}\u{1f}\u{7f}"`},
	}
	for _, tt := range tests {
		if got := QuoteString(tt.in); got != tt.want {
			t.Errorf("QuoteString(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	if got := FormatKey("port"); got != "port" {
		t.Errorf("FormatKey(\"port\") = %q, want \"port\"", got)
	}
	if got := FormatKey("true"); got != `"true"` {
		t.Errorf("FormatKey(\"true\") = %q, want \"\\\"true\\\"\"", got)
	}

	if got := FormatRefBody([]string{"app", "v1/beta", "db.host"}); got != `app/"v1/beta"/db.host` {
		t.Errorf("FormatRefBody = %q, want %q", got, `app/"v1/beta"/db.host`)
	}
}

func TestScanString(t *testing.T) {
	t.Run("ValidEscapes", func(t *testing.T) {
		src := []byte(`"hello \"\\\n\r\t\u{41}\u{1f600}"`)
		res := ScanString(src, 0)
		if res.ErrCode != "" {
			t.Fatalf("unexpected error: %+v", res)
		}
		want := "hello \"\\\n\r\tA😀"
		if res.Text != want || res.Length != len(src) {
			t.Errorf("ScanString = (%q, %d), want (%q, %d)", res.Text, res.Length, want, len(src))
		}
	})

	t.Run("InvalidEscapesAndChars", func(t *testing.T) {
		errCases := []struct {
			src     []byte
			wantErr string
		}{
			{[]byte(`"\x"`), "L003"},
			{[]byte(`"\u41"`), "L003"},
			{[]byte(`"\u{}"`), "L003"},
			{[]byte(`"\u{110000}"`), "L003"},
			{[]byte(`"\u{D800}"`), "L003"},
			{[]byte(`"\u{1234567}"`), "L003"},
			{[]byte(`"\u{41"`), "L003"},
			{[]byte("\"unterminated"), "L007"},
			{[]byte(`"trailing\`), "L007"},
			{[]byte("\"ctrl\x01\""), "L008"},
			{[]byte{'"', 0xff, '"'}, "L008"},
		}
		for _, tc := range errCases {
			res := ScanString(tc.src, 0)
			if res.ErrCode != tc.wantErr {
				t.Errorf("ScanString(%q).ErrCode = %q, want %q", tc.src, res.ErrCode, tc.wantErr)
			}
		}
	})
}

func TestScanRefAndParseRefBody(t *testing.T) {
	res := ScanRef([]byte(`&service/"http/2"/port`), 0)
	if res.ErrCode != "" {
		t.Fatalf("ScanRef unexpected error: %+v", res)
	}
	wantSegs := []string{"service", "http/2", "port"}
	if !slices.Equal(res.Segs, wantSegs) {
		t.Errorf("ScanRef.Segs = %v, want %v", res.Segs, wantSegs)
	}

	badRefs := []struct {
		src     string
		wantErr string
	}{
		{"&", "L009"},
		{"&a/", "L009"},
		{"&/a", "L009"},
		{`&"bad\x"`, "L003"},
	}
	for _, tc := range badRefs {
		r := ScanRef([]byte(tc.src), 0)
		if r.ErrCode != tc.wantErr {
			t.Errorf("ScanRef(%q).ErrCode = %q, want %q", tc.src, r.ErrCode, tc.wantErr)
		}
	}

	segs, ok := ParseRefBody(`a/"b/c"/d`)
	if !ok || !slices.Equal(segs, []string{"a", "b/c", "d"}) {
		t.Errorf("ParseRefBody failed: segs=%v, ok=%v", segs, ok)
	}
	if _, ok := ParseRefBody("a/b extra"); ok {
		t.Errorf("ParseRefBody should reject trailing characters")
	}
}
