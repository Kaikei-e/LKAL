package lkal

import "errors"

func ToJSON(f *File, plain bool) ([]byte, error) {
	return nil, errors.New("lkal: ToJSON is not implemented")
}

func FromJSON(src []byte) (*File, error) {
	return nil, &ErrorList{}
}
