package syntax

import "testing"

func TestCharPredicates(t *testing.T) {
	for _, b := range []byte{'=', '{', '}', '[', ']', ','} {
		if !IsSymbol(b) {
			t.Errorf("IsSymbol(%q) = false, want true", b)
		}
		if !IsDelimiter(b) {
			t.Errorf("IsDelimiter(%q) = false, want true", b)
		}
	}
	for _, b := range []byte{' ', '\t', '"', '#'} {
		if IsSymbol(b) {
			t.Errorf("IsSymbol(%q) = true, want false", b)
		}
		if !IsDelimiter(b) {
			t.Errorf("IsDelimiter(%q) = false, want true", b)
		}
	}
	if IsDelimiter('a') || IsSymbol('a') {
		t.Errorf("'a' should not be a delimiter or symbol")
	}

	if got := DelimiterRun([]byte("foo=bar"), 0); got != 3 {
		t.Errorf("DelimiterRun(\"foo=bar\", 0) = %d, want 3", got)
	}
	if got := DelimiterRun([]byte("foo"), 0); got != 3 {
		t.Errorf("DelimiterRun(\"foo\", 0) = %d, want 3", got)
	}

	if !IsNameStart('a') || !IsNameStart('Z') || !IsNameStart('_') || IsNameStart('-') || IsNameStart('0') {
		t.Errorf("unexpected IsNameStart behavior")
	}
	if !IsNameChar('a') || !IsNameChar('9') || !IsNameChar('-') || !IsNameChar('_') || IsNameChar('.') {
		t.Errorf("unexpected IsNameChar behavior")
	}
	if !IsRefSegmentChar('.') || !IsRefSegmentChar('-') || !IsRefSegmentChar('0') || IsRefSegmentChar('/') {
		t.Errorf("unexpected IsRefSegmentChar behavior")
	}
	for _, b := range []byte("0123456789abcdefABCDEF") {
		if !IsHexDigit(b) {
			t.Errorf("IsHexDigit(%q) = false, want true", b)
		}
	}
	if IsHexDigit('g') || IsHexDigit('G') {
		t.Errorf("IsHexDigit accepted non-hex byte")
	}
}

func TestControlAndRawCandidate(t *testing.T) {
	if !IsControlChar(0x00) || !IsControlChar(0x1f) || !IsControlChar(0x7f) {
		t.Errorf("expected C0 and DEL to be control chars")
	}
	if IsControlChar('\t') || IsControlChar(' ') || IsControlChar('あ') {
		t.Errorf("unexpected control char classification")
	}

	if IsRawCandidate("single line") {
		t.Errorf("single line should not be raw candidate")
	}
	if !IsRawCandidate("line1\nline2") {
		t.Errorf("multi-line with LF should be raw candidate")
	}
	if !IsRawCandidate("line1\n\tline2") {
		t.Errorf("multi-line with LF and tab should be raw candidate")
	}
	if IsRawCandidate("line1\r\nline2") {
		t.Errorf("string with CR should not be raw candidate")
	}
}

func TestKeyAndNameValidators(t *testing.T) {
	validEnums := []string{"tcp", "_private", "A1_b-2"}
	for _, s := range validEnums {
		if !IsValidEnumName(s) {
			t.Errorf("IsValidEnumName(%q) = false, want true", s)
		}
	}
	invalidEnums := []string{"", "-abc", "1abc", "a.b", "a/b"}
	for _, s := range invalidEnums {
		if IsValidEnumName(s) {
			t.Errorf("IsValidEnumName(%q) = true, want false", s)
		}
	}

	if !IsBareKey("host_name") || !IsBareKey("api-v1") {
		t.Errorf("expected valid bare keys")
	}
	for _, k := range []string{"", "true", "false", "123", "a.b", "$key"} {
		if IsBareKey(k) {
			t.Errorf("IsBareKey(%q) = true, want false", k)
		}
	}

	if !IsBareRefSegment("v1.0-alpha_1") {
		t.Errorf("expected valid bare ref segment")
	}
	for _, seg := range []string{"", "a/b", "a b", `"quoted"`} {
		if IsBareRefSegment(seg) {
			t.Errorf("IsBareRefSegment(%q) = true, want false", seg)
		}
	}
}
