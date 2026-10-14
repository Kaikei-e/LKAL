package main

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"

	lkal "github.com/Kaikei-e/LKAL"
)

type jsonErrorEntry struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

// formatErrorReport is a pure function that formats errors as text or JSON bytes.
func formatErrorReport(errs []*lkal.Error, jsonOutput bool) []byte {
	if len(errs) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if jsonOutput {
		entries := make([]jsonErrorEntry, len(errs))
		for i, e := range errs {
			entries[i] = jsonErrorEntry{
				File:    e.File,
				Line:    e.Pos.Line,
				Col:     e.Pos.Col,
				Code:    e.Code,
				Message: e.Msg,
				Hint:    e.Hint,
			}
		}
		_ = json.MarshalWrite(&buf, entries, jsontext.AllowInvalidUTF8(true))
		buf.WriteByte('\n')
		return buf.Bytes()
	}
	for _, e := range errs {
		fmt.Fprintln(&buf, e)
	}
	return buf.Bytes()
}

func reportErrors(errs []*lkal.Error, jsonOutput bool, stderr io.Writer) {
	if out := formatErrorReport(errs, jsonOutput); len(out) > 0 {
		_, _ = stderr.Write(out)
	}
}

func toErrors(err error) []*lkal.Error {
	return err.(*lkal.ErrorList).Errors
}

// withFileName is a pure function returning a new error slice with File set to fileName.
func withFileName(errs []*lkal.Error, fileName string) []*lkal.Error {
	res := make([]*lkal.Error, len(errs))
	for i, e := range errs {
		cp := *e
		cp.File = fileName
		res[i] = &cp
	}
	return res
}
