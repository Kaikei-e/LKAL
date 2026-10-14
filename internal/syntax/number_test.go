package syntax

import (
	"strings"
	"testing"
)

func TestScanPlainNumber(t *testing.T) {
	tests := []struct {
		in       string
		wantDig  int
		wantFrac bool
		wantOK   bool
	}{
		{"0", 1, false, true},
		{"-0", 1, false, true},
		{"42", 2, false, true},
		{"-12345", 5, false, true},
		{"0.25", 3, true, true},
		{"-12.340", 5, true, true},
		{"", 0, false, false},
		{"-", 0, false, false},
		{"00", 0, false, false},
		{"01", 0, false, false},
		{"1.", 0, false, false},
		{".5", 0, false, false},
		{"10ms", 0, false, false},
	}

	for _, tt := range tests {
		dig, frac, ok := ScanPlainNumber(tt.in)
		if dig != tt.wantDig || frac != tt.wantFrac || ok != tt.wantOK {
			t.Errorf("ScanPlainNumber(%q) = (%d, %v, %v), want (%d, %v, %v)",
				tt.in, dig, frac, ok, tt.wantDig, tt.wantFrac, tt.wantOK)
		}
	}
}

func TestValidateNumber(t *testing.T) {
	d100 := strings.Repeat("9", 99) + ".9"
	d101 := strings.Repeat("9", 100) + ".9"

	tests := []struct {
		in       string
		wantKind NumberKind
		wantErr  string
	}{
		{"0", NumberInt, ""},
		{"-9223372036854775808", NumberInt, ""},
		{"9223372036854775807", NumberInt, ""},
		{"9223372036854775808", NumberInvalid, "L005"},
		{"3.14", NumberDecimal, ""},
		{d100, NumberDecimal, ""},
		{d101, NumberInvalid, "L005"},
		{"512Mi", NumberQuantity, ""},
		{"0.5s", NumberQuantity, ""},
		{"1h30m", NumberInvalid, "L004"},
		{"10_000", NumberInvalid, "L009"},
		{"01", NumberInvalid, "L009"},
		{"1.2.3", NumberInvalid, "L009"},
		{"-", NumberInvalid, "L009"},
	}

	for _, tt := range tests {
		kind, errCode := ValidateNumber(tt.in)
		if kind != tt.wantKind || errCode != tt.wantErr {
			t.Errorf("ValidateNumber(%q) = (%v, %q), want (%v, %q)",
				tt.in, kind, errCode, tt.wantKind, tt.wantErr)
		}
	}
}

func TestNormalizeNumberAndSplitQuantity(t *testing.T) {
	normTests := []struct {
		in   string
		want string
	}{
		{"0", "0"},
		{"-0", "0"},
		{"42", "42"},
		{"0.0", "0.0"},
		{"-0.000", "0.0"},
		{"1.500", "1.5"},
		{"1.000", "1.0"},
		{"-2.50", "-2.5"},
	}
	for _, tt := range normTests {
		if got := NormalizeNumber(tt.in); got != tt.want {
			t.Errorf("NormalizeNumber(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	splitTests := []struct {
		in       string
		wantNum  string
		wantUnit string
	}{
		{"512Mi", "512", "Mi"},
		{"-0.5ms", "-0.5", "ms"},
		{"100", "100", ""},
	}
	for _, tt := range splitTests {
		num, unit := SplitQuantity(tt.in)
		if num != tt.wantNum || unit != tt.wantUnit {
			t.Errorf("SplitQuantity(%q) = (%q, %q), want (%q, %q)",
				tt.in, num, unit, tt.wantNum, tt.wantUnit)
		}
	}
}
