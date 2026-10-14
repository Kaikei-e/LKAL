package parser_test

import . "github.com/Kaikei-e/LKAL"

import "testing"

func TestSemanticErrors(t *testing.T) {
	tests := []errTestCase{
		{
			name:     "L201_DuplicateAssignKey",
			src:      "@lkal \"0.1\"\na = 1\na = 2\n",
			wantCode: "L201",
			wantPos:  errPos(3, 1),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_DuplicateRecordKey",
			src:      "@lkal \"0.1\"\nrec {}\nrec {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 1),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_DuplicateCollectionLabel",
			src:      "@lkal \"0.1\"\ncoll \"a\" {}\ncoll \"a\" {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 6),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L201_DuplicateUnicodeLabel",
			src:      "@lkal \"0.1\"\nsvc \"日本語\" {}\nsvc \"日本語\" {}\n",
			wantCode: "L201",
			wantPos:  errPos(3, 5),
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L202_AssignThenRecord",
			src:      "@lkal \"0.1\"\nk = 1\nk {}\n",
			wantCode: "L202",
			wantPos:  errPos(3, 1),
			wantMsg:  "key used for more than one kind of entry",
			wantHint: "use the key for one kind only: assignment, record or collection",
		},
		{
			name:     "L202_RecordThenCollection",
			src:      "@lkal \"0.1\"\nk {}\nk \"a\" {}\n",
			wantCode: "L202",
			wantPos:  errPos(3, 1),
			wantMsg:  "key used for more than one kind of entry",
			wantHint: "use the key for one kind only: assignment, record or collection",
		},
		{
			name:     "L202_CollectionThenAssign",
			src:      "@lkal \"0.1\"\nk \"a\" {}\nk = 1\n",
			wantCode: "L202",
			wantPos:  errPos(3, 1),
			wantMsg:  "key used for more than one kind of entry",
			wantHint: "use the key for one kind only: assignment, record or collection",
		},
		{
			name:     "L203_SeparatedByAssign",
			src:      "@lkal \"0.1\"\nport \"http\" {}\nhost = \"localhost\"\nport \"grpc\" {}\n",
			wantCode: "L203",
			wantPos:  errPos(4, 1),
			wantMsg:  "collection elements are not contiguous",
			wantHint: "write the elements next to each other",
		},
		{
			name:     "L203_SeparatedByRecord",
			src:      "@lkal \"0.1\"\nw \"a\" {}\nm {}\nw \"b\" {}\n",
			wantCode: "L203",
			wantPos:  errPos(4, 1),
			wantMsg:  "collection elements are not contiguous",
			wantHint: "write the elements next to each other",
		},
		{
			name:     "L203_SeparatedInNestedRecord",
			src:      "@lkal \"0.1\"\nbox {\n  item \"a\" {}\n  cnt = 1\n  item \"b\" {}\n}\n",
			wantCode: "L203",
			wantPos:  errPos(5, 3),
			wantMsg:  "collection elements are not contiguous",
			wantHint: "write the elements next to each other",
		},
		{
			name:     "L204_IntAndString",
			src:      "@lkal \"0.1\"\nl = [1, \"two\"]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 9),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L204_IntAndDecimal",
			src:      "@lkal \"0.1\"\nl = [1, 2.5]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 9),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L204_DecimalAndInt",
			src:      "@lkal \"0.1\"\nl = [1.5, 2]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 11),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L204_BoolAndEnum",
			src:      "@lkal \"0.1\"\nl = [true, .active]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 12),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L204_StringAndRef",
			src:      "@lkal \"0.1\"\nl = [\"a\", &ref/one]\n",
			wantCode: "L204",
			wantPos:  errPos(2, 11),
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L205_BareDollarKey",
			src:      "@lkal \"0.1\"\n\"$foo\" = 1\n",
			wantCode: "L205",
			wantPos:  errPos(2, 1),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L205_QuotedDollarKey",
			src:      "@lkal \"0.1\"\n\"$schema\" = \"test\"\n",
			wantCode: "L205",
			wantPos:  errPos(2, 1),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L205_DollarRecordKey",
			src:      "@lkal \"0.1\"\n\"$rec\" {}\n",
			wantCode: "L205",
			wantPos:  errPos(2, 1),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L205_DollarCollectionKey",
			src:      "@lkal \"0.1\"\n\"$coll\" \"a\" {}\n",
			wantCode: "L205",
			wantPos:  errPos(2, 1),
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L206_CollectionAssign",
			src:      "@lkal \"0.1\"\nitem \"lbl\" = 1\n",
			wantCode: "L206",
			wantPos:  errPos(2, 12),
			wantMsg:  "value assigned to a collection element",
			wantHint: `write a block instead: key "label" { ... }`,
		},
		{
			name:     "L206_CollectionAssignSpaced",
			src:      "@lkal \"0.1\"\nitem \"lbl\"   = 1\n",
			wantCode: "L206",
			wantPos:  errPos(2, 14),
			wantMsg:  "value assigned to a collection element",
			wantHint: `write a block instead: key "label" { ... }`,
		},
		{
			name:     "L207_Version02",
			src:      "@lkal \"0.2\"\n",
			wantCode: "L207",
			wantPos:  errPos(1, 7),
			wantMsg:  "unsupported version",
			wantHint: `this implementation accepts only "0.1"`,
		},
		{
			name:     "L207_Version10",
			src:      "@lkal \"1.0\"\n",
			wantCode: "L207",
			wantPos:  errPos(1, 7),
			wantMsg:  "unsupported version",
			wantHint: `this implementation accepts only "0.1"`,
		},
		{
			name:     "L207_Version010",
			src:      "@lkal \"0.10\"\n",
			wantCode: "L207",
			wantPos:  errPos(1, 7),
			wantMsg:  "unsupported version",
			wantHint: `this implementation accepts only "0.1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse(tt.name, []byte(tt.src))
			if err == nil {
				t.Fatalf("expected error, got file: %v", file)
			}
			errCheckSingle(t, err, tt.wantCode, tt.wantPos, tt.wantMsg, tt.wantHint, tt.checkNonEmptyHint)
		})
	}

	t.Run("ValidQuantityDifferentUnitsInList", func(t *testing.T) {
		file, err := Parse("units.lkal", []byte("@lkal \"0.1\"\nl = [10m, 20s]\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file == nil {
			t.Fatalf("expected non-nil File")
		}
	})

	t.Run("ValidEmptyList", func(t *testing.T) {
		file, err := Parse("empty_list.lkal", []byte("@lkal \"0.1\"\nl = []\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file == nil {
			t.Fatalf("expected non-nil File")
		}
	})

	t.Run("ValidDollarLabel", func(t *testing.T) {
		file, err := Parse("dollar_label.lkal", []byte("@lkal \"0.1\"\nsvc \"$schema\" {}\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file == nil {
			t.Fatalf("expected non-nil File")
		}
	})
}

func TestSemanticPrecedence(t *testing.T) {
	t.Run("L205_Over_L202", func(t *testing.T) {
		src := "@lkal \"0.1\"\n\"$k\" = 1\n\"$k\" {}\n"
		file, err := Parse("prec205_202.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 2 {
			t.Fatalf("expected exactly 2 errors, got %d: %v", len(errList.Errors), errList.Error())
		}
		errAssertError(t, errList.Errors[0], "L205", errPos(2, 1), "key starts with $", "rename the key", false)
		errAssertError(t, errList.Errors[1], "L205", errPos(3, 1), "key starts with $", "rename the key", false)
	})

	t.Run("L205_Over_L201", func(t *testing.T) {
		src := "@lkal \"0.1\"\n\"$dup\" = 1\n\"$dup\" = 2\n"
		file, err := Parse("prec205_201.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 2 {
			t.Fatalf("expected exactly 2 errors, got %d: %v", len(errList.Errors), errList.Error())
		}
		errAssertError(t, errList.Errors[0], "L205", errPos(2, 1), "key starts with $", "rename the key", false)
		errAssertError(t, errList.Errors[1], "L205", errPos(3, 1), "key starts with $", "rename the key", false)
	})

	t.Run("L202_Over_L201", func(t *testing.T) {
		src := "@lkal \"0.1\"\nitem = 1\nitem {}\n"
		file, err := Parse("prec202_201.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errCheckSingle(t, err, "L202", errPos(3, 1), "key used for more than one kind of entry", "use the key for one kind only: assignment, record or collection", false)
	})

	t.Run("L201_Over_L203", func(t *testing.T) {
		src := "@lkal \"0.1\"\nport \"http\" {}\nhost = \"localhost\"\nport \"http\" {}\n"
		file, err := Parse("prec201_203.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errCheckSingle(t, err, "L201", errPos(4, 6), "duplicate key or label", "remove one of them", false)
	})
}
