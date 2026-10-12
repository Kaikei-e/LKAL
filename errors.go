package lkal

import (
	"sort"
	"strconv"
	"strings"
)

type Pos struct {
	Line int
	Col  int
}

func (p Pos) String() string {
	return strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col)
}

type Error struct {
	File string
	Pos  Pos
	Code string
	Msg  string
	Hint string
}

func (e *Error) Error() string {
	s := e.File + ":" + e.Pos.String() + ": " + e.Code + " " + e.Msg
	if e.Hint != "" {
		s += "\n  hint: " + e.Hint
	}
	return s
}

type ErrorList struct {
	Errors []*Error
}

func (l *ErrorList) Error() string {
	if len(l.Errors) == 0 {
		return ""
	}
	var b strings.Builder
	for i, err := range l.Errors {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

var errorMessages = map[string]string{
	"L001": "# か ## で始まるコメントは書けない",
	"L002": "行末やリストの内側にはコメントを書けない",
	"L003": "使えないエスケープ",
	"L004": "単位の後ろに数字が続いている",
	"L005": "数値が範囲を超えている",
	"L006": "使えないディレクティブ",
	"L007": "閉じていない文字列",
	"L008": "使えない文字",
	"L009": "トークンの形が正しくない",
	"L101": "見出し行の途中で改行している",
	"L102": "値の位置に裸の単語は書けない",
	"L103": "閉じていない括弧",
	"L104": "複数行文字列の途中に空行かコメントがある",
	"L105": "版の宣言がない、先頭にない、または2回以上ある",
	"L106": "予期しないトークン",
	"L201": "キーまたはラベルが重複している",
	"L202": "1つのキーを複数の種類に使っている",
	"L203": "コレクションの要素が連続していない",
	"L204": "型の混ざったリスト",
	"L205": "$ で始まるキーは書けない",
	"L206": "コレクションの要素に値を代入している",
	"L207": "対応していない版",
}

var errorHints = map[string]string{
	"L001": "### で始める",
	"L002": "リストの外の、独立した行に移す",
	"L003": `\u{…} を使う`,
	"L004": "単位を1つにする（例: 90m）",
	"L005": "整数は int64 の範囲、数値は100桁以内に収める",
	"L006": "v0.1で使えるのは @lkal だけ",
	"L007": `同じ行で " を閉じる。複数行の文字列は \\ 行で書く`,
	"L008": `文字を消す。制御文字は \u{…} で書く`,
	"L009": "数値・列挙子・参照・ディレクティブの書き方に合わせる",
	"L101": "Key・Label・{ を同じ行に書く",
	"L102": `文字列なら "WORD"、列挙子なら .WORD と書く`,
	"L103": "対応する閉じ括弧を書く",
	"L104": `間を詰める。空の行は \\ だけの行で書く`,
	"L105": `先頭に @lkal "0.1" を1回だけ書く`,
	"L106": "項目は キー = 値、キー { … }、キー \"ラベル\" { … } のどれかで始める",
	"L201": "片方を消す",
	"L202": "代入・レコード・コレクションのどれか1種類にそろえる",
	"L203": "要素をまとめて書く",
	"L204": "要素を同じ型にそろえる",
	"L205": "キー名を変える",
	"L206": `key "label" { … } のようにブロックで書く`,
	"L207": `このバージョンが受け付けるのは "0.1" だけ`,
}

func newError(file string, pos Pos, code, hint string) *Error {
	if hint == "" {
		hint = errorHints[code]
	}
	return &Error{
		File: file,
		Pos:  pos,
		Code: code,
		Msg:  errorMessages[code],
		Hint: hint,
	}
}

func sortErrors(errs []*Error) []*Error {
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Pos.Line != errs[j].Pos.Line {
			return errs[i].Pos.Line < errs[j].Pos.Line
		}
		if errs[i].Pos.Col != errs[j].Pos.Col {
			return errs[i].Pos.Col < errs[j].Pos.Col
		}
		return errs[i].Code < errs[j].Code
	})
	if len(errs) > 20 {
		return errs[:20]
	}
	return errs
}
