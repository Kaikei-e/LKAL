package lkal

func lex(name string, src []byte) ([]token, []*Error) {
	return []token{
		{
			kind: tokEOF,
			pos:  Pos{Line: 1, Col: 1},
			lit:  "",
		},
	}, nil
}
