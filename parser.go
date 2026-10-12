package lkal

func Parse(name string, src []byte) (*File, error) {
	return nil, &ErrorList{
		Errors: []*Error{
			{
				File: name,
				Pos:  Pos{Line: 1, Col: 1},
				Code: "L105",
				Msg:  errorMessages["L105"],
				Hint: errorHints["L105"],
			},
		},
	}
}

func (f *File) Version() string {
	return ""
}

func (f *File) Data() Record {
	return Record{}
}
