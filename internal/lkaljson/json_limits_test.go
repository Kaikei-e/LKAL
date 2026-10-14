package lkaljson_test

import . "github.com/Kaikei-e/LKAL"

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFromJSONDepthLimit(t *testing.T) {
	tests := []struct {
		name       string
		bracketN   int
		wantL304   bool
		wantL301   bool
		wantErrors []string
		wantHint   string
		wantCol    int
	}{
		{
			name:       "depth_10000_ok",
			bracketN:   9999,
			wantL304:   true,
			wantErrors: []string{"L304 1:21"},
		},
		{
			name:     "depth_10001_exceeds_limit",
			bracketN: 10000,
			wantL301: true,
			wantCol:  10019,
			wantHint: "nest objects and arrays at most 10000 levels deep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `{"$lkal":"0.1","a":` + strings.Repeat("[", tt.bracketN) + strings.Repeat("]", tt.bracketN) + `}`
			f, err := FromJSON([]byte(input))
			if f != nil {
				t.Errorf("expected nil File, got: %v", f)
			}
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}

			if tt.wantL304 {
				got := jsonTExtractErrors(errList)
				if !reflect.DeepEqual(got, tt.wantErrors) {
					t.Errorf("error mismatch:\ngot:  %v\nwant: %v", got, tt.wantErrors)
				}
			}

			if tt.wantL301 {
				var foundL301 *Error
				for _, e := range errList.Errors {
					if e != nil && e.Code == "L301" {
						foundL301 = e
						break
					}
				}
				if foundL301 == nil {
					t.Fatalf("expected L301 in errors, got: %v", jsonTExtractErrors(errList))
				}
				wantPos := Pos{Line: 1, Col: tt.wantCol}
				if foundL301.Pos != wantPos {
					t.Errorf("L301 Pos mismatch: got %v, want %v", foundL301.Pos, wantPos)
				}
				if foundL301.Hint != tt.wantHint {
					t.Errorf("L301 Hint mismatch: got %q, want %q", foundL301.Hint, tt.wantHint)
				}
			}
		})
	}
}

func TestFromJSONNestingLimit(t *testing.T) {
	const (
		recPrefix  = `{"$lkal":"0.1",`
		recUnit    = `"a":{`
		elemPrefix = `{"$lkal":"0.1",`
		elemUnit   = `"c":{"$coll":{"l":{`
		hint1000   = "nest blocks at most 1000 levels deep"
	)

	recCol1001 := jsonTComputeBlockBraceCol(recPrefix, recUnit, 1001)
	elemCol1001 := jsonTComputeBlockBraceCol(elemPrefix, elemUnit, 1001)

	tests := []struct {
		name     string
		input    string
		wantOK   bool
		wantCode string
		wantCol  int
		wantHint string
	}{
		{
			name:   "records_1000_levels_succeeds",
			input:  recPrefix + strings.Repeat(recUnit, 1000) + strings.Repeat("}", 1000) + "}",
			wantOK: true,
		},
		{
			name:     "records_1001_levels_l107",
			input:    recPrefix + strings.Repeat(recUnit, 1001) + strings.Repeat("}", 1001) + "}",
			wantOK:   false,
			wantCode: "L107",
			wantCol:  recCol1001,
			wantHint: hint1000,
		},
		{
			name:     "elements_1001_levels_l107",
			input:    elemPrefix + strings.Repeat(elemUnit, 1001) + strings.Repeat("}}}", 1001) + "}",
			wantOK:   false,
			wantCode: "L107",
			wantCol:  elemCol1001,
			wantHint: hint1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := FromJSON([]byte(tt.input))
			if tt.wantOK {
				if err != nil {
					t.Fatalf("FromJSON failed unexpectedly: got %v, want nil", err)
				}
				if f == nil {
					t.Fatalf("FromJSON returned nil File: got nil, want non-nil")
				}
				if gotVer, wantVer := f.Version(), "0.1"; gotVer != wantVer {
					t.Errorf("Version mismatch: got %q, want %q", gotVer, wantVer)
				}
				return
			}

			if err == nil {
				t.Fatalf("FromJSON succeeded unexpectedly: got nil error, want %s error, file: %v", tt.wantCode, f)
			}
			if f != nil {
				t.Errorf("FromJSON returned non-nil File on error: got %v, want nil", f)
			}

			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList: got %T (%v), want *ErrorList", err, err)
			}
			if errList == nil || len(errList.Errors) == 0 {
				t.Fatalf("ErrorList is empty: got 0 errors, want 1 error")
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("error count mismatch: got %d, want 1: %v", len(errList.Errors), jsonTExtractErrors(errList))
			}

			first := errList.Errors[0]
			if first == nil {
				t.Fatalf("first error is nil: got nil, want non-nil Error")
			}
			if first.Code != tt.wantCode {
				t.Errorf("Code mismatch: got %q, want %q", first.Code, tt.wantCode)
			}
			wantPos := Pos{Line: 1, Col: tt.wantCol}
			if first.Pos != wantPos {
				t.Errorf("Pos mismatch: got %v, want %v", first.Pos, wantPos)
			}
			if first.Hint != tt.wantHint {
				t.Errorf("Hint mismatch: got %q, want %q", first.Hint, tt.wantHint)
			}
		})
	}
}

func TestFromJSONManyErrorsLinear(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling test in short mode")
	}

	buildInput := func(k int) []byte {
		return []byte(`{"$lkal":"0.1","a":[` + strings.Repeat("null,", k) + "null]}")
	}

	checkErrors := func(name string, src []byte) {
		f, err := FromJSON(src)
		if f != nil {
			t.Errorf("%s: FromJSON returned non-nil File on error: %v", name, f)
		}
		if err == nil {
			t.Fatalf("%s: expected error, got nil", name)
		}
		errList, ok := err.(*ErrorList)
		if !ok {
			t.Fatalf("%s: expected *ErrorList, got %T: %v", name, err, err)
		}
		if len(errList.Errors) != 20 {
			t.Fatalf("%s: error count mismatch: got %d, want 20", name, len(errList.Errors))
		}
		for i, e := range errList.Errors {
			if e == nil {
				t.Fatalf("%s: error[%d] is nil", name, i)
			}
			if e.Code != "L302" {
				t.Errorf("%s: error[%d] code mismatch: got %q, want %q", name, i, e.Code, "L302")
			}
		}
	}

	measureMin := func(src []byte) time.Duration {
		var minDur time.Duration
		for i := 0; i < 3; i++ {
			runtime.GC()
			start := time.Now()
			_, _ = FromJSON(src)
			d := time.Since(start)
			if i == 0 || d < minDur {
				minDur = d
			}
		}
		return minDur
	}

	baseN := 100000
	inputN := buildInput(baseN)
	input4N := buildInput(baseN * 4)

	checkErrors("N_20000", inputN)
	checkErrors("4N_80000", input4N)

	durN := measureMin(inputN)
	dur4N := measureMin(input4N)

	if dur4N > 8*durN {
		ratio := float64(dur4N)
		if durN > 0 {
			ratio /= float64(durN)
			t.Errorf("4n duration %v exceeds 8x n duration %v (ratio: %.2fx)", dur4N, durN, ratio)
		} else {
			t.Errorf("4n duration %v exceeds 8x n duration %v", dur4N, durN)
		}
	}
}
