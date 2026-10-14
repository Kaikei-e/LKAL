package parser

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
)

func TestParseScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling test in short mode")
	}

	buildInput := func(prefix, repeat, suffix string, k int) []byte {
		return []byte(prefix + strings.Repeat(repeat, k) + suffix)
	}

	measureMin := func(src []byte) time.Duration {
		var minDur time.Duration
		for i := 0; i < 3; i++ {
			runtime.GC()
			start := time.Now()
			_, _ = Parse("scale.lkal", src)
			d := time.Since(start)
			if i == 0 || d < minDur {
				minDur = d
			}
		}
		return minDur
	}

	families := []struct {
		name   string
		prefix string
		repeat string
		suffix string
		baseK  int
		build  func(k int) []byte
	}{
		{
			name:   "long_valid_line",
			prefix: "@lkal \"0.1\"\na = [",
			repeat: "1, ",
			suffix: "1]\n",
			baseK:  174762,
		},
		{
			name:   "long_line_illegal_characters",
			prefix: "@lkal \"0.1\"\na = ",
			repeat: "%",
			suffix: "\n",
			baseK:  524288,
		},
		{
			name:   "failed_entry_skip_long_block",
			prefix: "@lkal \"0.1\"\na {\n  = 1\n",
			repeat: "  b = 1\n",
			suffix: "}\nc = 1\n",
			baseK:  65536,
		},
		{
			name:   "many_sibling_blocks",
			prefix: "@lkal \"0.1\"\nr {\n",
			repeat: "b { x = 1 }\n",
			suffix: "}\n",
			baseK:  65536,
		},
		{
			name:  "deep_nesting",
			baseK: 20000,
			build: func(k int) []byte {
				return []byte("@lkal \"0.1\"\na = 1\n" + strings.Repeat("a { ", k) + "x = 1" + strings.Repeat(" }", k) + "\n")
			},
		},
	}

	for _, fam := range families {
		t.Run(fam.name, func(t *testing.T) {
			var inputN, input4N []byte
			if fam.build != nil {
				inputN = fam.build(fam.baseK)
				input4N = fam.build(fam.baseK * 4)
			} else {
				inputN = buildInput(fam.prefix, fam.repeat, fam.suffix, fam.baseK)
				input4N = buildInput(fam.prefix, fam.repeat, fam.suffix, fam.baseK*4)
			}

			durN := measureMin(inputN)
			dur4N := measureMin(input4N)

			if dur4N > 8*durN {
				ratio := float64(dur4N)
				if durN > 0 {
					ratio /= float64(durN)
					t.Errorf("%s: 4n duration %v exceeds 8x n duration %v (ratio: %.2fx)", fam.name, dur4N, durN, ratio)
				} else {
					t.Errorf("%s: 4n duration %v exceeds 8x n duration %v", fam.name, dur4N, durN)
				}
			}
		})
	}
}

type nestTExpectedErr struct {
	code     string
	line     int
	col      int
	wantMsg  string
	wantHint string
}

type nestTParserCase struct {
	name     string
	src      string
	wantOK   bool
	wantErrs []nestTExpectedErr
}

func TestParseNestingLimit(t *testing.T) {
	tests := []nestTParserCase{
		{
			name:   "1000_nested_record_blocks_ok",
			src:    "@lkal \"0.1\"\n" + strings.Repeat("a { ", 1000) + "x = 1" + strings.Repeat(" }", 1000) + "\n",
			wantOK: true,
		},
		{
			name:   "1001_nested_record_blocks_fail",
			src:    "@lkal \"0.1\"\n" + strings.Repeat("a { ", 1001) + "x = 1" + strings.Repeat(" }", 1001) + "\nb = c\n",
			wantOK: false,
			wantErrs: []nestTExpectedErr{
				{
					code:     "L107",
					line:     2,
					col:      4003,
					wantMsg:  "blocks nested too deeply",
					wantHint: "nest blocks at most 1000 levels deep",
				},
				{
					code: "L102",
					line: 3,
					col:  5,
				},
			},
		},
		{
			name:   "1001_nested_element_blocks_fail",
			src:    "@lkal \"0.1\"\n" + strings.Repeat("a \"l\" { ", 1001) + "x = 1" + strings.Repeat(" }", 1001) + "\nb = c\n",
			wantOK: false,
			wantErrs: []nestTExpectedErr{
				{
					code:     "L107",
					line:     2,
					col:      8007,
					wantMsg:  "blocks nested too deeply",
					wantHint: "nest blocks at most 1000 levels deep",
				},
				{
					code: "L102",
					line: 3,
					col:  5,
				},
			},
		},
		{
			name:   "1001_nested_multiline_record_blocks_fail",
			src:    "@lkal \"0.1\"\n" + strings.Repeat("a {\n", 1001) + "x = 1\n" + strings.Repeat("}\n", 1001),
			wantOK: false,
			wantErrs: []nestTExpectedErr{
				{
					code:     "L107",
					line:     1002,
					col:      3,
					wantMsg:  "blocks nested too deeply",
					wantHint: "nest blocks at most 1000 levels deep",
				},
			},
		},
		{
			name:   "100000_nested_levels_single_line_no_panic",
			src:    "@lkal \"0.1\"\n" + strings.Repeat("a { ", 100000) + "x = 1" + strings.Repeat(" }", 100000) + "\n",
			wantOK: false,
			wantErrs: []nestTExpectedErr{
				{
					code:     "L107",
					line:     2,
					col:      4003,
					wantMsg:  "blocks nested too deeply",
					wantHint: "nest blocks at most 1000 levels deep",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var file *ast.File
			var parseErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Parse(%q) panicked: got %v, want no panic", tc.name, r)
					}
				}()
				file, parseErr = Parse(tc.name+".lkal", []byte(tc.src))
			}()

			if tc.wantOK {
				if parseErr != nil {
					t.Fatalf("Parse(%q) failed: got error %v, want nil", tc.name, parseErr)
				}
				if file == nil {
					t.Fatalf("Parse(%q) returned nil File: got nil, want non-nil *File", tc.name)
				}
				return
			}

			if parseErr == nil {
				t.Fatalf("Parse(%q) succeeded: got nil error, want error", tc.name)
			}
			if file != nil {
				t.Errorf("Parse(%q) returned non-nil File on error: got %v, want nil", tc.name, file)
			}
			errList, ok := parseErr.(*diag.ErrorList)
			if !ok {
				t.Fatalf("Parse(%q) returned error type: got %T, want *ErrorList", tc.name, parseErr)
			}
			if errList == nil {
				t.Fatalf("Parse(%q) returned nil *ErrorList: got nil, want non-nil", tc.name)
			}
			if len(errList.Errors) != len(tc.wantErrs) {
				t.Fatalf("Parse(%q) error count: got %d (%v), want %d", tc.name, len(errList.Errors), errList.Error(), len(tc.wantErrs))
			}
			for i, want := range tc.wantErrs {
				got := errList.Errors[i]
				if got == nil {
					t.Fatalf("Parse(%q) error[%d]: got nil, want %s at %d:%d", tc.name, i, want.code, want.line, want.col)
				}
				if got.Code != want.code || got.Pos.Line != want.line || got.Pos.Col != want.col {
					t.Errorf("Parse(%q) error[%d]: got %s at %d:%d, want %s at %d:%d",
						tc.name, i, got.Code, got.Pos.Line, got.Pos.Col, want.code, want.line, want.col)
				}
				if want.wantMsg != "" && got.Msg != want.wantMsg {
					t.Errorf("Parse(%q) error[%d] Msg: got %q, want %q", tc.name, i, got.Msg, want.wantMsg)
				}
				if want.wantHint != "" && got.Hint != want.wantHint {
					t.Errorf("Parse(%q) error[%d] Hint: got %q, want %q", tc.name, i, got.Hint, want.wantHint)
				}
			}
		})
	}
}
