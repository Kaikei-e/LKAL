package lkal

func (f *File) Get(path string) (Value, error) { return nil, &ErrorList{} }
func (f *File) Set(path string, v Value) error { return &ErrorList{} }
func (f *File) Delete(path string) error       { return &ErrorList{} }
func ParseValue(src []byte) (Value, error)     { return nil, &ErrorList{} }
