package schema

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Kaikei-e/LKAL"
)

func fuzzSchemaIsCompileCode(code string) bool {
	switch code {
	case "L001", "L002", "L003", "L004", "L005", "L006", "L007", "L008", "L009",
		"L101", "L102", "L103", "L104", "L105", "L106", "L107",
		"L201", "L204", "L205", "L207",
		"L601", "L602", "L603", "L604", "L605", "L606", "L607":
		return true
	default:
		return false
	}
}

func fuzzSchemaIsValidateCode(code string) bool {
	switch code {
	case "L501", "L502", "L503", "L504", "L505", "L506", "L507", "L508", "L509", "L510":
		return true
	default:
		return false
	}
}

func fuzzSchemaErrorsOrdered(prev, curr *lkal.Error) bool {
	lp, lc := prev.Pos.Line > 0, curr.Pos.Line > 0
	if lp != lc {
		return lp
	}
	if lp {
		if prev.Pos.Line != curr.Pos.Line {
			return prev.Pos.Line < curr.Pos.Line
		}
		if prev.Pos.Col != curr.Pos.Col {
			return prev.Pos.Col < curr.Pos.Col
		}
		if prev.Code != curr.Code {
			return prev.Code < curr.Code
		}
		return prev.Path <= curr.Path
	}
	if prev.Path != curr.Path {
		return prev.Path < curr.Path
	}
	return prev.Code <= curr.Code
}

func fuzzCheckSchemaErrorList(
	t *testing.T,
	err error,
	wantFile string,
	src []byte,
	expectUnlocated bool,
	isKnownCode func(string) bool,
) *lkal.ErrorList {
	t.Helper()
	var errList *lkal.ErrorList
	if !errors.As(err, &errList) || errList == nil {
		t.Fatalf("expected non-nil *lkal.ErrorList, got %T: %v", err, err)
	}
	if len(errList.Errors) < 1 || len(errList.Errors) > 20 {
		t.Errorf("error count %d not in [1, 20]", len(errList.Errors))
	}
	if errList.Error() == "" {
		t.Errorf("ErrorList.Error() returned empty string")
	}

	var lines [][]byte
	maxLine := 0
	if !expectUnlocated && src != nil {
		maxLine = bytes.Count(src, []byte("\n")) + 1
		lines = bytes.Split(src, []byte("\n"))
	}

	for i, e := range errList.Errors {
		if e == nil {
			t.Fatalf("nil error at index %d", i)
		}
		if e.File != wantFile {
			t.Errorf("error[%d].File = %q, want %q", i, e.File, wantFile)
		}
		if !isKnownCode(e.Code) {
			t.Errorf("error[%d] unexpected code %q", i, e.Code)
		}
		if e.Msg == "" {
			t.Errorf("error[%d] empty Msg for code %q", i, e.Code)
		}
		if e.Hint == "" {
			t.Errorf("error[%d] empty Hint for code %q", i, e.Code)
		}
		if e.Error() == "" {
			t.Errorf("error[%d].Error() is empty for code %q", i, e.Code)
		}
		if expectUnlocated {
			if e.Pos != (lkal.Pos{}) {
				t.Errorf("error[%d] expected zero Pos, got %v", i, e.Pos)
			}
		} else if e.Pos.Line > 0 {
			if e.Pos.Line > maxLine {
				t.Errorf("error[%d] Pos.Line %d > maxLine %d", i, e.Pos.Line, maxLine)
			}
			if e.Pos.Line <= len(lines) {
				maxCol := 1 + utf8.RuneCount(lines[e.Pos.Line-1])
				if e.Pos.Col < 1 || e.Pos.Col > maxCol {
					t.Errorf("error[%d] Pos.Col %d on line %d not in [1, %d]", i, e.Pos.Col, e.Pos.Line, maxCol)
				}
			} else if e.Pos.Col < 1 {
				t.Errorf("error[%d] Pos.Col %d < 1", i, e.Pos.Col)
			}
		} else if e.Pos.Col != 0 {
			t.Errorf("error[%d] Pos.Line == 0 but Pos.Col == %d", i, e.Pos.Col)
		}

		if i > 0 {
			prev := errList.Errors[i-1]
			if !fuzzSchemaErrorsOrdered(prev, e) {
				t.Errorf("errors not sorted at %d: prev=(%v,%s,%q) curr=(%v,%s,%q)",
					i, prev.Pos, prev.Code, prev.Path, e.Pos, e.Code, e.Path)
			}
		}
	}
	return errList
}

func fuzzSchemaLoadSeeds(patterns ...string) [][]byte {
	var out [][]byte
	for _, pat := range patterns {
		matches, _ := filepath.Glob(pat)
		for _, p := range matches {
			data, err := os.ReadFile(p)
			if err == nil && len(data) <= 8192 {
				out = append(out, data)
			}
		}
	}
	return out
}

func FuzzCompile(f *testing.F) {
	f.Add(metaBytes)
	extraSeeds := [][]byte{
		[]byte(""),
		[]byte("@lkal \"0.1\"\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot {}\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.3\"\nunknown = 1\nroot {}\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = 2\nroot {}\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"a\" { use = &def/a }\nroot {}\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"a\" { use = &def/b }\ndef \"b\" { use = &def/c }\ndef \"c\" { use = &def/a }\nroot {}\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"u\" { types = [.int, .decimal], min = 0.5, max = 10, default = 1 } }\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"e\" { type = .enum, values = [] } }\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"m\" { type = .map, unique-by = [\"id\"], value { type = .string } } }\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"p\" { type = .string, pattern = \"(?P<bad\" } }\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { field \"r\" { type = .ref, min-segments = 3, max-segments = 1 } }\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\nroot { reserved = [\"x\"], field \"x\" { type = .bool } }\n"),
		[]byte("@lkal \"0.1\"\nlkal-schema = \"0.2\"\ndef \"n\" { type = .record, field \"next\" { use = &def/n } }\nroot { field \"head\" { use = &def/n } }\n"),
		[]byte("\xff\xfe\xfd\x00\x01"),
	}
	for _, s := range extraSeeds {
		f.Add(s)
	}
	for _, s := range fuzzSchemaLoadSeeds(
		filepath.Join("..", "testdata", "schema", "valid", "*.schema.lkal"),
		filepath.Join("..", "testdata", "schema", "invalid", "*.schema.lkal"),
		filepath.Join("..", "testdata", "schema", "schema-invalid", "*.schema.lkal"),
	) {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := Compile("fuzz.schema.lkal", data)
		if err == nil {
			if s == nil {
				t.Fatalf("Compile returned nil Schema with nil error")
			}
			_ = s.ID()
			_ = s.Version()
			_ = s.ValidateValue(lkal.Record{})
			_, _ = s.Default("a")
			return
		}
		if s != nil {
			t.Errorf("Compile returned non-nil Schema on error")
		}
		_ = fuzzCheckSchemaErrorList(t, err, "fuzz.schema.lkal", data, false, fuzzSchemaIsCompileCode)
	})
}

type fuzzByteReader struct {
	data []byte
	off  int
}

func (r *fuzzByteReader) next() byte {
	idx := r.off
	r.off++
	if len(r.data) == 0 {
		return byte(idx*37 + 11)
	}
	if idx < len(r.data) {
		return r.data[idx]
	}
	return r.data[idx%len(r.data)] ^ byte((idx/len(r.data))*31+13)
}

func (r *fuzzByteReader) n(k int) int {
	if k <= 1 {
		return 0
	}
	return int(r.next()) % k
}

func fuzzLoadFixedSchema(t testing.TB, name string) *Schema {
	t.Helper()
	p := filepath.Join("..", "testdata", "schema", "valid", name+".schema.lkal")
	src, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read fixed schema %s: %v", p, err)
	}
	s, err := Compile(name+".schema.lkal", src)
	if err != nil {
		t.Fatalf("compile fixed schema %s: %v", name, err)
	}
	return s
}

func fuzzSynthValidAndCorrupt(data []byte, devSchema, envSchema, svcSchema *Schema) (*Schema, lkal.Record, lkal.Record, string, string) {
	r := &fuzzByteReader{data: data}
	choice := r.n(3)

	switch choice {
	case 0: // dev schema
		vMaj := r.n(10)
		vMin := r.n(20)
		vPat := r.n(30)
		ver := fmt.Sprintf("%d.%d.%d", vMaj, vMin, vPat)
		basePort := 2000 + r.n(40000)

		makeMember := func(off int) lkal.Record {
			return lkal.Record{
				Fields: []lkal.Field{
					{Key: "client-url", Value: lkal.String(fmt.Sprintf("http://127.0.0.1:%d", basePort+off*2))},
					{Key: "peer-url", Value: lkal.String(fmt.Sprintf("http://127.0.0.1:%d", basePort+off*2+1))},
				},
			}
		}
		valid := lkal.Record{
			Fields: []lkal.Field{
				{
					Key: "etcd",
					Value: lkal.Record{
						Fields: []lkal.Field{
							{Key: "version", Value: lkal.String(ver)},
							{
								Key: "member",
								Value: lkal.Record{
									Fields: []lkal.Field{
										{Key: "m0", Value: makeMember(0)},
										{Key: "m1", Value: makeMember(1)},
										{Key: "m2", Value: makeMember(2)},
									},
								},
							},
						},
					},
				},
			},
		}

		corrupt := normalizeAndClone(valid).(lkal.Record)
		etcdRec := corrupt.Fields[0].Value.(lkal.Record)
		switch r.n(4) {
		case 0: // L505 pattern mismatch on version
			etcdRec.Fields[0].Value = lkal.String("bad-ver")
			corrupt.Fields[0].Value = etcdRec
			return devSchema, valid, corrupt, "L505", "etcd/version"
		case 1: // L504 min-entries violation
			memRec := etcdRec.Fields[1].Value.(lkal.Record)
			memRec.Fields = memRec.Fields[:2]
			etcdRec.Fields[1].Value = memRec
			corrupt.Fields[0].Value = etcdRec
			return devSchema, valid, corrupt, "L504", "etcd/member"
		case 2: // L510 invalid map key
			memRec := etcdRec.Fields[1].Value.(lkal.Record)
			memRec.Fields[0].Key = "bad key!"
			etcdRec.Fields[1].Value = memRec
			corrupt.Fields[0].Value = etcdRec
			return devSchema, valid, corrupt, "L510", `etcd/member/"bad key!"`
		default: // L503 unknown field
			etcdRec.Fields = append(etcdRec.Fields, lkal.Field{Key: "extra", Value: lkal.Int(1)})
			corrupt.Fields[0].Value = etcdRec
			return devSchema, valid, corrupt, "L503", "etcd/extra"
		}

	case 1: // envelope schema
		states := []lkal.Enum{lkal.Enum("true"), lkal.Enum("false"), lkal.Enum("unknown")}
		state := states[r.n(len(states))]
		rev := int64(1 + r.n(100))
		valid := lkal.Record{
			Fields: []lkal.Field{
				{
					Key: "meta",
					Value: lkal.Record{
						Fields: []lkal.Field{
							{Key: "group", Value: lkal.String("apps.nayutar.io")},
							{Key: "kind", Value: lkal.String("workload")},
							{Key: "name", Value: lkal.String(fmt.Sprintf("app-%d", r.n(1000)))},
							{Key: "revision", Value: lkal.Int(rev)},
						},
					},
				},
				{
					Key: "spec",
					Value: lkal.Record{
						Fields: []lkal.Field{
							{Key: "anything", Value: lkal.Int(int64(r.n(50)))},
						},
					},
				},
				{
					Key: "status",
					Value: lkal.Record{
						Fields: []lkal.Field{
							{Key: "observedGeneration", Value: lkal.Int(1)},
							{
								Key: "condition",
								Value: lkal.Record{
									Fields: []lkal.Field{
										{
											Key: "Ready",
											Value: lkal.Record{
												Fields: []lkal.Field{
													{Key: "state", Value: state},
													{Key: "reason", Value: lkal.String("Available")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		corrupt := normalizeAndClone(valid).(lkal.Record)
		switch r.n(4) {
		case 0: // L502 missing required meta/kind
			metaRec := corrupt.Fields[0].Value.(lkal.Record)
			metaRec.Fields = append(metaRec.Fields[:1], metaRec.Fields[2:]...)
			corrupt.Fields[0].Value = metaRec
			return envSchema, valid, corrupt, "L502", "meta/kind"
		case 1: // L504 revision < 1
			metaRec := corrupt.Fields[0].Value.(lkal.Record)
			metaRec.Fields[3].Value = lkal.Int(0)
			corrupt.Fields[0].Value = metaRec
			return envSchema, valid, corrupt, "L504", "meta/revision"
		case 2: // L506 bad enum state
			stRec := corrupt.Fields[2].Value.(lkal.Record)
			condMap := stRec.Fields[1].Value.(lkal.Record)
			readyRec := condMap.Fields[0].Value.(lkal.Record)
			readyRec.Fields[0].Value = lkal.Enum("maybe")
			condMap.Fields[0].Value = readyRec
			stRec.Fields[1].Value = condMap
			corrupt.Fields[2].Value = stRec
			return envSchema, valid, corrupt, "L506", "status/condition/Ready/state"
		default: // L501 wrong type for observedGeneration
			stRec := corrupt.Fields[2].Value.(lkal.Record)
			stRec.Fields[0].Value = lkal.String("one")
			corrupt.Fields[2].Value = stRec
			return envSchema, valid, corrupt, "L501", "status/observedGeneration"
		}

	default: // service_full schema
		var share lkal.Value = lkal.Int(int64(r.n(2)))
		if r.n(2) == 1 {
			share = lkal.Decimal(fmt.Sprintf("0.%d", 1+r.n(9)))
		}
		portNum := int64(1 + r.n(65535))
		valid := lkal.Record{
			Fields: []lkal.Field{
				{
					Key: "service",
					Value: lkal.Record{
						Fields: []lkal.Field{
							{
								Key: "web",
								Value: lkal.Record{
									Fields: []lkal.Field{
										{Key: "traffic-share", Value: share},
										{
											Key: "container",
											Value: lkal.Record{
												Fields: []lkal.Field{
													{
														Key: "app",
														Value: lkal.Record{
															Fields: []lkal.Field{
																{Key: "image", Value: lkal.String("img:1")},
																{Key: "memory", Value: lkal.Quantity{Number: "512", Unit: "Mi"}},
																{Key: "secret", Value: lkal.Ref{"secret", "tls"}},
																{
																	Key: "port",
																	Value: lkal.Record{
																		Fields: []lkal.Field{
																			{
																				Key: "http",
																				Value: lkal.Record{
																					Fields: []lkal.Field{
																						{Key: "number", Value: lkal.Int(portNum)},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
										{
											Key: "plugin",
											Value: lkal.Record{
												Fields: []lkal.Field{
													{Key: "p1", Value: lkal.Record{Fields: []lkal.Field{{Key: "order", Value: lkal.Int(1)}}}},
													{Key: "p2", Value: lkal.Record{Fields: []lkal.Field{{Key: "order", Value: lkal.Int(2)}}}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		corrupt := normalizeAndClone(valid).(lkal.Record)
		svcMap := corrupt.Fields[0].Value.(lkal.Record)
		webRec := svcMap.Fields[0].Value.(lkal.Record)
		switch r.n(4) {
		case 0: // L507 bad quantity unit
			cMap := webRec.Fields[1].Value.(lkal.Record)
			appRec := cMap.Fields[0].Value.(lkal.Record)
			appRec.Fields[1].Value = lkal.Quantity{Number: "512", Unit: "Mx"}
			cMap.Fields[0].Value = appRec
			webRec.Fields[1].Value = cMap
			svcMap.Fields[0].Value = webRec
			corrupt.Fields[0].Value = svcMap
			return svcSchema, valid, corrupt, "L507", "service/web/container/app/memory"
		case 1: // L508 bad ref target
			cMap := webRec.Fields[1].Value.(lkal.Record)
			appRec := cMap.Fields[0].Value.(lkal.Record)
			appRec.Fields[2].Value = lkal.Ref{"config", "tls"}
			cMap.Fields[0].Value = appRec
			webRec.Fields[1].Value = cMap
			svcMap.Fields[0].Value = webRec
			corrupt.Fields[0].Value = svcMap
			return svcSchema, valid, corrupt, "L508", "service/web/container/app/secret"
		case 2: // L509 duplicate unique-by order
			pMap := webRec.Fields[2].Value.(lkal.Record)
			p2Rec := pMap.Fields[1].Value.(lkal.Record)
			p2Rec.Fields[0].Value = lkal.Int(1)
			pMap.Fields[1].Value = p2Rec
			webRec.Fields[2].Value = pMap
			svcMap.Fields[0].Value = webRec
			corrupt.Fields[0].Value = svcMap
			return svcSchema, valid, corrupt, "L509", "service/web/plugin/p2/order"
		default: // L501 wrong type on union field traffic-share
			webRec.Fields[0].Value = lkal.Bool(true)
			svcMap.Fields[0].Value = webRec
			corrupt.Fields[0].Value = svcMap
			return svcSchema, valid, corrupt, "L501", "service/web/traffic-share"
		}
	}
}

func fuzzRecordToFile(t *testing.T, rec lkal.Record) *lkal.File {
	t.Helper()
	var b strings.Builder
	b.WriteString("@lkal \"0.1\"\n\n")
	for _, f := range rec.Fields {
		b.WriteString(formatKey(f.Key))
		if _, isRec := f.Value.(lkal.Record); isRec {
			b.WriteByte(' ')
		} else {
			b.WriteString(" = ")
		}
		b.WriteString(string(lkal.FormatValue(f.Value)))
		b.WriteByte('\n')
	}
	file, err := lkal.Parse("synth.lkal", []byte(b.String()))
	if err != nil {
		t.Fatalf("Parse synthesized record failed: %v\nsrc:\n%s", err, b.String())
	}
	return file
}

func FuzzValidate(f *testing.F) {
	devSchema := fuzzLoadFixedSchema(f, "dev")
	envSchema := fuzzLoadFixedSchema(f, "envelope")
	svcSchema := fuzzLoadFixedSchema(f, "service_full")
	fixedSchemas := []struct {
		name   string
		schema *Schema
	}{
		{"dev", devSchema},
		{"envelope", envSchema},
		{"service_full", svcSchema},
	}

	extraSeeds := [][]byte{
		[]byte("@lkal \"0.1\"\n"),
		[]byte("@lkal \"0.1\"\netcd { version = \"bad\", replica = 1 }\n"),
		[]byte("@lkal \"0.1\"\nmeta { kind = \"app\", name = \"x\" }\nstatus { condition \"bad\" { state = .nope, reason = \"!\" } }\n"),
		[]byte("@lkal \"0.1\"\nservice \"web\" { replicas = -1, traffic-share = 2.5, plugin \"a\" { order = 1 }, plugin \"b\" { order = 1 } }\n"),
		[]byte("@lkal \"0.1\"\nservice { web { container { app { image = \"\", memory = 12Ti, secret = &other/x } } } }\n"),
		{0, 1, 2, 3, 4, 5, 6, 7},
		{10, 20, 30, 40, 50},
		{255, 128, 64, 32, 16},
	}
	for _, s := range extraSeeds {
		f.Add(s)
	}
	for _, s := range fuzzSchemaLoadSeeds(
		filepath.Join("..", "testdata", "schema", "valid", "*.lkal"),
		filepath.Join("..", "testdata", "schema", "invalid", "*.lkal"),
	) {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Part A: arbitrary LKAL document against fixed schemas.
		if doc, err := lkal.Parse("fuzz.lkal", data); err == nil {
			fmtBefore := slices.Clone(doc.Format())
			jsonBefore, err := lkal.ToJSON(doc, false)
			if err != nil {
				t.Fatalf("ToJSON failed on parsed doc: %v", err)
			}
			docFmt, err := lkal.Parse("fuzz.lkal", fmtBefore)
			if err != nil {
				t.Fatalf("Parse(Format) failed: %v", err)
			}
			docJSON, err := lkal.FromJSON(jsonBefore)
			if err != nil {
				t.Fatalf("FromJSON(ToJSON) failed: %v", err)
			}
			origData := doc.Data()
			origDataFmt := slices.Clone(lkal.FormatValue(origData))

			for _, fs := range fixedSchemas {
				s := fs.schema
				valErr := s.Validate(doc)

				// Non-mutation of File.
				if !bytes.Equal(doc.Format(), fmtBefore) {
					t.Errorf("%s: Validate mutated doc.Format()", fs.name)
				}
				jsonAfter, err := lkal.ToJSON(doc, false)
				if err != nil || !bytes.Equal(jsonAfter, jsonBefore) {
					t.Errorf("%s: Validate mutated ToJSON(doc, false): err=%v", fs.name, err)
				}

				var fileErrCount int
				if valErr != nil {
					el := fuzzCheckSchemaErrorList(t, valErr, "fuzz.lkal", data, false, fuzzSchemaIsValidateCode)
					fileErrCount = len(el.Errors)
				}

				valValErr := s.ValidateValue(origData)
				if !bytes.Equal(lkal.FormatValue(origData), origDataFmt) {
					t.Errorf("%s: ValidateValue mutated input Value", fs.name)
				}
				var valErrCount int
				if valValErr != nil {
					el := fuzzCheckSchemaErrorList(t, valValErr, "", nil, true, fuzzSchemaIsValidateCode)
					valErrCount = len(el.Errors)
				}

				// Format invariance: exact (Code, Path) slice preserved.
				valFmtErr := s.Validate(docFmt)
				if valFmtErr != nil {
					_ = fuzzCheckSchemaErrorList(t, valFmtErr, "fuzz.lkal", fmtBefore, false, fuzzSchemaIsValidateCode)
				}
				if !slices.Equal(extractCodePathSlice(t, valFmtErr), extractCodePathSlice(t, valErr)) {
					t.Errorf("%s: Format invariance violated:\norig: %v\nfmt:  %v",
						fs.name, extractCodePathSlice(t, valErr), extractCodePathSlice(t, valFmtErr))
				}

				// LKAL-JSON round-trip invariance.
				valJSONErr := s.ValidateValue(docJSON.Data())
				if valJSONErr != nil {
					_ = fuzzCheckSchemaErrorList(t, valJSONErr, "", nil, true, fuzzSchemaIsValidateCode)
				}
				if !slices.Equal(extractCodePathSlice(t, valJSONErr), extractCodePathSlice(t, valValErr)) {
					t.Errorf("%s: ValidateValue(FromJSON(ToJSON).Data()) != ValidateValue(doc.Data())", fs.name)
				}
				if fileErrCount < maxErrors && valErrCount < maxErrors {
					if !slices.Equal(extractCodePathSorted(t, valJSONErr), extractCodePathSorted(t, valErr)) {
						t.Errorf("%s: JSON round-trip (Code, Path) set mismatch:\nfile: %v\njson: %v",
							fs.name, extractCodePathSorted(t, valErr), extractCodePathSorted(t, valJSONErr))
					}
				} else if (fileErrCount == maxErrors) != (valErrCount == maxErrors) {
					t.Errorf("%s: cap mismatch between Validate (%d) and ValidateValue (%d)", fs.name, fileErrCount, valErrCount)
				}

				// Complete idempotence and non-mutation.
				c1, compErr := s.Complete(origData)
				if compErr != nil {
					t.Fatalf("%s: Complete(origData) failed unexpectedly: %v", fs.name, compErr)
				}
				if !bytes.Equal(lkal.FormatValue(origData), origDataFmt) {
					t.Errorf("%s: Complete mutated input Value", fs.name)
				}
				c2, compErr2 := s.Complete(c1)
				if compErr2 != nil {
					t.Fatalf("%s: Complete(c1) failed unexpectedly: %v", fs.name, compErr2)
				}
				if !lkal.Equal(c2, c1) {
					t.Errorf("%s: Complete is not idempotent", fs.name)
				}
			}
		}

		// Part B: structured valid value vs single-field corrupted value.
		s, validRec, corruptRec, wantCode, wantPath := fuzzSynthValidAndCorrupt(data, devSchema, envSchema, svcSchema)
		if err := s.ValidateValue(validRec); err != nil {
			t.Fatalf("structured valid record failed ValidateValue: %v\nval: %s", err, lkal.FormatValue(validRec))
		}
		validFile := fuzzRecordToFile(t, validRec)
		if err := s.Validate(validFile); err != nil {
			t.Fatalf("structured valid file failed Validate: %v", err)
		}
		completed, err := s.Complete(validRec)
		if err != nil {
			t.Fatalf("Complete(validRec) failed: %v", err)
		}
		if err := s.ValidateValue(completed); err != nil {
			t.Fatalf("ValidateValue(Complete(validRec)) failed: %v", err)
		}

		corruptValErr := s.ValidateValue(corruptRec)
		if corruptValErr == nil {
			t.Fatalf("corrupted record unexpectedly passed ValidateValue (expected %s at %s)", wantCode, wantPath)
		}
		valPairs := extractCodePathSlice(t, corruptValErr)
		if !slices.Contains(valPairs, codePathPair{code: wantCode, path: wantPath}) {
			t.Errorf("corrupted record ValidateValue missing (%s, %s); got %v", wantCode, wantPath, valPairs)
		}

		corruptFile := fuzzRecordToFile(t, corruptRec)
		corruptFileErr := s.Validate(corruptFile)
		if corruptFileErr == nil {
			t.Fatalf("corrupted file unexpectedly passed Validate (expected %s at %s)", wantCode, wantPath)
		}
		filePairs := extractCodePathSlice(t, corruptFileErr)
		if !slices.Contains(filePairs, codePathPair{code: wantCode, path: wantPath}) {
			t.Errorf("corrupted file Validate missing (%s, %s); got %v", wantCode, wantPath, filePairs)
		}
	})
}
