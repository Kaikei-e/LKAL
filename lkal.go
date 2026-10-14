package lkal

import (
	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/format"
	"github.com/Kaikei-e/LKAL/internal/lkaljson"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/parser"
	lkalpath "github.com/Kaikei-e/LKAL/internal/path"
)

// Data model types exported via type aliases.
type (
	Value      = model.Value
	String     = model.String
	Int        = model.Int
	Decimal    = model.Decimal
	Bool       = model.Bool
	Enum       = model.Enum
	Quantity   = model.Quantity
	Ref        = model.Ref
	List       = model.List
	Record     = model.Record
	Collection = model.Collection
	Field      = model.Field
	Elem       = model.Elem
)

// Diagnostic types exported via type aliases.
type (
	Pos       = diag.Pos
	Error     = diag.Error
	ErrorList = diag.ErrorList
)

// File represents a parsed LKAL document, preserving comments and formatting layout.
type File struct {
	doc *ast.File
}

// Parse lexes, parses, and validates src, returning a *File or *ErrorList.
func Parse(name string, src []byte) (*File, error) {
	doc, err := parser.Parse(name, src)
	if err != nil {
		return nil, err
	}
	return &File{doc: doc}, nil
}

// Version returns the declared LKAL version of f (e.g., "0.1").
func (f *File) Version() string {
	if f == nil || f.doc == nil {
		return ""
	}
	return f.doc.Version
}

// Data returns the pure data model Record of f.
func (f *File) Data() Record {
	if f == nil || f.doc == nil {
		return Record{}
	}
	return ast.BuildRecord(f.doc.Body)
}

// Format returns the canonical LKAL representation of f.
func (f *File) Format() []byte {
	if f == nil || f.doc == nil {
		return nil
	}
	return format.FormatFile(f.doc)
}

// Get returns the value at path within f.
func (f *File) Get(path string) (Value, error) {
	return lkalpath.Get(f.doc.Name, f.doc.Body, path)
}

// Set updates the value at path within f, preserving surrounding comments and layout.
func (f *File) Set(path string, v Value) error {
	newBody, err := lkalpath.Set(f.doc.Name, f.doc.Body, path, v)
	if err != nil {
		return err
	}
	f.doc.Body = newBody
	return nil
}

// Delete removes the entry or collection element at path within f.
func (f *File) Delete(path string) error {
	newBody, err := lkalpath.Delete(f.doc.Name, f.doc.Body, path)
	if err != nil {
		return err
	}
	f.doc.Body = newBody
	return nil
}

// Equal reports whether two LKAL values are semantically equal.
func Equal(a, b Value) bool {
	return model.Equal(a, b)
}

// Format parses src and returns its canonical LKAL representation.
func Format(src []byte) ([]byte, error) {
	return format.Format(src)
}

// FormatValue returns the canonical text of one Value without a trailing newline.
func FormatValue(v Value) []byte {
	return format.FormatValue(v)
}

// ParseValue parses a standalone LKAL value expression into a Value.
func ParseValue(src []byte) (Value, error) {
	return parser.ParseValue(src)
}

// ToJSON encodes f into LKAL-JSON (plain == false) or plain JSON (plain == true).
func ToJSON(f *File, plain bool) ([]byte, error) {
	var doc *ast.File
	if f != nil {
		doc = f.doc
	}
	return lkaljson.ToJSON(doc, plain)
}

// FromJSON parses LKAL-JSON bytes into a *File.
func FromJSON(src []byte) (*File, error) {
	doc, err := lkaljson.FromJSON(src)
	if err != nil {
		return nil, err
	}
	return &File{doc: doc}, nil
}
