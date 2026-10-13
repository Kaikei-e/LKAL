---
title: "go 行を 1.27 に上げ、LKAL-JSON の読み書きに encoding/json/jsontext を使う"
status: accepted
date: 2026-10-13
depends-on: ["0001", "0004"]
---

# 0008: go 行を 1.27 に上げ、LKAL-JSON の読み書きに encoding/json/jsontext を使う

## コンテキスト

[[0004]] は、`FromJSON` の JSON を自前で走査すると決めた。当時の判断材料は次の 2 つである。

- `encoding/json`（v1）の `SyntacticError.Offset` は、誤りの種類によって指す位置が違った。
- `encoding/json/v2` と `encoding/json/jsontext` は、Go 1.26 では `GOEXPERIMENT=jsonv2` を付けたときだけ使えた。[[0001]] で `go.mod` の `go` 行を 1.26 にしているので、これらは使えなかった。

この方針で実装した `json.go` は約 1,650 行になった。実装レビューでは、契約からのずれが約 8 件見つかった。部分的なリテラルの位置、構文エラーの前に見つけた誤りが消えること、などである。誤りの数に対して二乗の時間がかかる位置計算も見つかった。

その後、Go 1.27（2026-08-19）で `encoding/json/v2` と `encoding/json/jsontext` が正式に使えるようになった（ユーザーの指摘、2026-10-13）。手元のツールチェーンは go1.27.0〜1.27.2 である。

## 決定

1. `go.mod` の `go` 行を 1.27 にする。[[0001]] の「`go` 行は 1.26」をこの ADR で改める。外部モジュールを `require` しない方針は変えない。
2. `FromJSON` は `jsontext.Decoder` でトークンを順に読む。
    - 重複キーは L201 として自分で報告し、読み進めたいので、`jsontext.AllowDuplicateNames(true)` を付ける。
    - それ以外は既定のまま使う。不正な UTF-8 と、対になっていないサロゲートは、構文エラーになる。
    - 構文エラーは L301 にし、位置は `SyntacticError.ByteOffset` から求める。
    - 値ごとの誤り（L302 など）の位置は、直前の `InputOffset` から空白と `,` `:` を読み飛ばした、トークンの先頭とする。
3. L301 の位置の規則を、jsontext の振る舞いに合わせて次のように改める。ほかの言語の実装も同じ位置を出せるように、規則として書く。

    | 誤り | 位置 |
    | --- | --- |
    | 不正なエスケープ（未知の文字、16 進数の誤り、対になっていないサロゲート、入力の末尾で切れたエスケープ） | そのエスケープの `\` |
    | 不正な UTF-8（入力の末尾で切れた複数バイトの並びも含む） | その並びの先頭のバイト |
    | 値やキーが来るべき位置の `,`（`[1,]`、`[,1]`、`{"a":1,,}`） | その `,` |
    | 入れ子が 10000 段を超える | 10001 段目を開く括弧（hint は入れ子の上限を示す） |
    | それ以外 | 入力が正しい JSON の接頭辞でなくなる最初のバイト。途中で入力が終わったときは入力の末尾 |

4. `ToJSON` は `jsontext.Encoder` に `jsontext.WithIndent("  ")` を付けて、トークンを順に書く。レイアウトは [[0004]] と同じで、`json.MarshalIndent` と同じ形に末尾の改行 1 つが付く。文字列のエスケープは jsontext の既定（RFC 8785 の最小限のエスケープ）に従う。U+2028 と U+2029 はエスケープしない。`<` `>` `&` もそのまま書く。
5. CLI の `--json` の出力にも v2（`encoding/json/v2` か `jsontext`）を使う。
6. [[0004]] のうち、「JSON を自前で走査する」（決定 7）と「`encoding/json` と同じく U+2028 と U+2029 をエスケープする」は、この ADR で改める。コード L301〜L305 と、それ以外の規則は変えない。

## 根拠（調査結果・出典）

- Go 1.27 のリリースノート（[go.dev/doc/go1.27](https://go.dev/doc/go1.27#jsonv2)）。`encoding/json/v2` と `encoding/json/jsontext` を新しいパッケージとして載せている。`encoding/json` は v2 の実装の上で動き、互換性に問題があれば `GOEXPERIMENT=nojsonv2` で元の実装に戻せる、と書いている。最新のパッチは go1.27.2（[go.dev/doc/devel/release](https://go.dev/doc/devel/release#go1.27.2)）。調べたのは agy ワーカーで、検索結果の抜粋による。
- 手元の go1.27.2 で次を確かめた。
    - `GOEXPERIMENT` を指定しなくても `encoding/json/jsontext` を使うプログラムをビルドできる。
    - `GOEXPERIMENT=nojsonv2` を付けると「build constraints exclude all Go files」になる。ソースには `//go:build goexperiment.jsonv2` が残っており、既定で有効な実験という扱いである。
- 同じく go1.27.2 で、`jsontext.Decoder` の振る舞いを試した。
    - 数値トークンの `String()` は、書かれた字面を保つ（`1.50`、`-0`、`1e5`）。
    - `ByteOffset` は、上の表の規則どおりだった。例: `{"$lkal": "0.1", "a": }` は `}`、`{"a":tRue}` は `R`、`{"a":tru` は入力の末尾、`{"a":"\q"}` と `{"a":"\uD800\uDC0G"}` は `\`、`[1,]` は `,`、`"x\xff"` は `\xff`。
    - 入れ子は 10001 段目で「exceeded max depth」になった。
    - `jsontext.Encoder` の既定は、`<&>` と U+2028 をそのまま書き、`\b` `\f` を短い形にし、ほかの制御文字を `\u00XX` にする。空のオブジェクトと配列は `{}` / `[]` のまま 1 行で書く。
- `jsontext` のドキュメントの記述:
    - 「ByteOffset indicates that an error occurred at or after this byte offset」
    - 「InputOffset … gives the location of the next byte immediately after the most recently returned token or value」
    - 既定は RFC 7493（I-JSON）に従い、重複キーと不正な UTF-8 を拒否する（[pkg.go.dev/encoding/json/jsontext](https://pkg.go.dev/encoding/json/jsontext)）。

## 検討した代替案

- **自前の走査器を直して使い続ける**: 標準ライブラリが同じ仕事をするのに、約 1,650 行と、それに伴う位置の規則のずれを保守し続けることになる。jsontext は I-JSON の検査（重複キー、UTF-8、サロゲート）と入れ子の上限まで持っている。
- **`encoding/json`（v1）の `Decoder.Token` を使う**: [[0004]] で確かめたとおり、エラーの位置がそろわない。閉じていない入力も検出できない。
- **`go` 行を 1.26 のままにし、jsontext を使うファイルだけビルドタグで分ける**: Go 1.26 では `GOEXPERIMENT=jsonv2` が要り、実装が 2 つに分かれる。
- **L301 の位置の規則を [[0004]] のまま保ち、jsontext の位置を補正する**: エスケープの誤りを `\` で報告するのは、LKAL 自身の L003 と同じ考え方である。末尾のカンマをそのカンマで示すほうが、直し方も分かりやすい。補正のためのコードを持つ理由がない。

## 影響とトレードオフ

- 得るもの: `json.go` が大きく縮む。L301 の位置、I-JSON の検査、入れ子の上限は標準ライブラリに任せられる。
- 失うもの:
    - Go 1.26 以前ではビルドできない。
    - `GOEXPERIMENT=nojsonv2` で v1 の実装に戻した利用者も、LKAL をビルドできない。
- リスク: jsontext のドキュメントは「The contents of this error as produced by this package may change over time」と書いており、将来の Go で `ByteOffset` が変わるかもしれない。L301 の位置は適合テストで固定してあるので、変わればテストで分かる。そのときは規則を見直す。
- 今後: README の「Go 1.26 以上」を 1.27 に改める。

## 関連ADR

[[0001]]（`go` 行を改める）、[[0004]]（決定 7 と U+2028 / U+2029 のエスケープを改める）。
