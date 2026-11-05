# LKAL

[![CI](https://github.com/Kaikei-e/LKAL/actions/workflows/ci.yml/badge.svg)](https://github.com/Kaikei-e/LKAL/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Kaikei-e/LKAL.svg)](https://pkg.go.dev/github.com/Kaikei-e/LKAL)

[English](README.md) · [Design Principles](docs/PRINCIPLE.md) · [ADR](docs/ADR) · [Apache-2.0](LICENSE)

LKAL (Literals, Keys, and Labels) は、あるべき状態を宣言するための最小のデータ形式です。値の型は書き方だけで決まり、構造は `{}` だけで表し、ラベルで入れ子のレコードを圧縮することで、すべての項目を名前付きの `/` パスで一意に指せます。

```lkal
@lkal "0.1"

### フロントエンドのWebサーバー
service "web" {
  replicas = 3
  traffic-share = 0.25
  labels { "app.kubernetes.io/name" = "web", tier = "frontend" }

  container "app" {
    image = "ghcr.io/acme/web:1.4.2"
    args = ["--port", "8080"]
    memory = 512Mi
    timeout = 30s
    port "http" { number = 8080, protocol = .tcp }
    secret = &secret/db-creds
  }

  startup =
    \\#!/bin/sh
    \\exec /app/web --port 8080
}
```

このリポジトリは LKAL の Go リファレンス実装（意味規則の検査を含むパーサー、正準フォーマッタ、LKAL-JSON 変換、パス操作、`lkal` コマンド）を提供します。標準ライブラリのみで実装されています。仕様は **ドラフト v0.1** であり、変更される可能性があります。

## 値の型

| 書き方 | Go の型 | LKAL-JSON |
| --- | --- | --- |
| `"text"`, `\\` 行 | `String` (`string`) | `"text"` |
| `42` | `Int` (`int64`) | `42` |
| `0.25` | `Decimal`（正規化済み10進表記の `string`） | `0.25` |
| `true`, `false` | `Bool` (`bool`) | `true`, `false` |
| `.tcp` | `Enum`（先頭の `.` を除いた `string`） | `{"$enum": "tcp"}` |
| `512Mi`, `30s` | `Quantity` (`struct{ Number, Unit string }`) | `{"$qty": "512Mi"}` |
| `&secret/db-creds` | `Ref` (`[]string`) | `{"$ref": "secret/db-creds"}` |
| `["a", "b"]` | `List`（`[]Value`、同一型のスカラー） | `["a", "b"]` |
| `key { … }`, `key "label" { … }`, `key "label" = …` | `Record`（`struct{ Fields []Field }`、ラベルは入れ子のレコードに展開） | `{"key": {…}}` |

すべての文書は `@lkal "0.1"` で始まります（`L105`, `L207`）。コメントは各行を `###` で始め、複数行文字列は各行を `\\` で始めます。引用符付き文字列のエスケープは `\"`, `\\`, `\n`, `\r`, `\t`, `\u{HEX}` のみを受け付け（`L003`）、UTF-8 BOM と `\t` 以外の制御文字は拒否されます（`L008`）。数値は符号付き10進の整数（`int64`）と小数（最大100桁、`L005`）のみを受け付け、先頭の余分な `0`（`01`）、基数接頭辞（`0xFF`, `0b10`, `0o77`）、指数表記（`1e5`）、複合単位（`1h30m`, `L004`）はすべて拒否されます。値の位置の裸の単語（`true` と `false` を除く）、`null`、`$` で始まるキー・ラベル（`L205`）、および重複キーやパスの衝突（`L201`）もすべてエラーになります。ブロックとラベルの入れ子は最大 1000 段です（`L107`）。設計規則の詳細は [`docs/PRINCIPLE.md`](docs/PRINCIPLE.md) を参照してください。

## インストール

```sh
go get github.com/Kaikei-e/LKAL
go install github.com/Kaikei-e/LKAL/cmd/lkal@latest
```

Go 1.27.3 以降が必要です（標準ライブラリの `encoding/json/jsontext` および `encoding/json/v2` を使用）。

## 使い方

```go
import lkal "github.com/Kaikei-e/LKAL"

f, err := lkal.Parse("app.lkal", src)
if err != nil {
	log.Fatal(err) // *lkal.ErrorList
}

data := f.Data() // lkal.Record
```

`Parse`、`Format`、`FromJSON`、`ParseValue`、`Get`、`Set`、`Delete` はエラー時に `*lkal.ErrorList`（`Errors []*lkal.Error`）を返し、1 始まりの行番号と Unicode コードポイント列番号（`Pos{Line, Col}`）の順に最大 20 件のエラーを報告します。各エラーは固定のコードと修正のヒントを持ちます。

```text
bad.lkal:2:12: L102 bare word in value position
  hint: write "three" for a string or .three for an enum
```

エラーコードは段階ごとに分類されています。

| 範囲 | 段階 | コード |
| --- | --- | --- |
| `L001`–`L009` | 字句解析 | `###` コメント（`L001`, `L002`）、エスケープ（`L003`）、複合単位（`L004`）、`int64`・100桁上限（`L005`）、ディレクティブ（`L006`）、未閉鎖の文字列（`L007`）、不正文字・BOM（`L008`）、不正トークン（`L009`） |
| `L101`–`L107` | 構文解析 | 見出し行の改行（`L101`）、値の位置の裸の単語（`L102`）、未閉鎖の括弧（`L103`）、`\\` 行の途中の空行・コメント（`L104`）、`@lkal` 宣言（`L105`）、予期しないトークン（`L106`）、1000段超の入れ子（`L107`） |
| `L201`–`L207` | 意味規則 | 重複キーまたは経路衝突（`L201`）、型の混在リスト（`L204`）、`$` で始まるキー・ラベル（`L205`）、未対応の版（`L207`） |
| `L301`–`L305` | LKAL-JSON | 不正な JSON（`L301`）、`null`（`L302`）、指数表記（`L303`）、非スカラーのリスト要素（`L304`）、不正な `$enum` / `$qty` / `$ref` タグ（`L305`） |
| `L401`–`L403` | パス・値操作 | 不正なパス（`L401`）、存在しないパス（`L402`）、`Set` に渡した不正な `Value`（`L403`） |

残りの API はパース済みの `*lkal.File` または `lkal.Value` に対して動作します。

| 関数 | 用途 |
| --- | --- |
| `f.Version()`, `f.Data()` | 文書の版（`"0.1"`）と `Record` データモデルの取得 |
| `lkal.Format(src)`, `f.Format()` | 正準形への整形（冪等。コメント・空行・1行ブロック・ラベル記法を保持） |
| `lkal.ToJSON(f, plain)` | LKAL-JSON（`plain = false`、`"$lkal"` 付きの可逆変換）または一方向の plain JSON（`plain = true`、`$enum`・`$qty`・`$ref` を文字列に展開） |
| `lkal.FromJSON(src)` | LKAL-JSON から `*lkal.File` への復元 |
| `f.Get(path)`, `f.Set(path, v)`, `f.Delete(path)` | `/` パス（例: `service/web/container/app/image`）による値の取得・編集。コメントや未編集項目のレイアウトは保持される |
| `lkal.ParseValue(src)`, `lkal.FormatValue(v)` | `lkal get` や `lkal set` で使われる単一の値テキスト（スカラー・リスト・`\\` 行・`{ … }` レコード）の解析・整形 |
| `lkal.Equal(a, b)` | スキーマ非依存の意味的等価判定（レコードのキー順は無視し、リストの要素順は区別する） |

## コマンド

```text
lkal fmt [-w] [-l] [--json] FILE…
lkal check [--json] FILE…
lkal json [-plain] [--json] FILE
lkal from-json [--json] FILE
lkal get [-r] [--json] FILE PATH
lkal set [--json] FILE PATH VALUE
lkal del [--json] FILE PATH
```

すべてのコマンドで `--json`（または `-json`）を付けると、エラーを1行の JSON 配列（`[{"file","line","col","code","message","hint"}]`）として標準エラー出力に出します。`FILE` に `-` を渡すと標準入力から読み込みます（`set -` と `del -` は更新後の正準形 LKAL を標準出力に出し、`fmt -w -` はエラーになります）。`get -r` は `String`、`Enum`、`Ref` の値を引用符や先頭の `.` / `&` なしの生形式で出力します。終了コードは `0` が成功、`1` が LKAL のエラー、`2` が使い方または I/O のエラーです。

```sh
lkal set web.lkal service/web/container/app/image '"ghcr.io/acme/web:1.5.0"'
lkal get web.lkal service/web/container/app/port/http/number   # 8080
lkal get -r web.lkal service/web/container/app/image           # ghcr.io/acme/web:1.5.0
```

## 開発

```sh
go test ./...
go test -race -short -count=1 -shuffle=on ./...   # 高負荷環境でタイミング検査をスキップ
go vet ./...
gofmt -s -l .
go test -run '^$' -fuzz=FuzzParse -fuzztime=30s
go test -run '^$' -fuzz=FuzzFormat -fuzztime=30s
go test -run '^$' -fuzz=FuzzFromJSON -fuzztime=30s
go test -run '^$' -fuzz=FuzzEqual -fuzztime=30s
go test -run '^$' -fuzz=FuzzParseValue -fuzztime=30s
go test -run '^$' -fuzz=FuzzPath -fuzztime=30s
go test -run '^$' -fuzz=FuzzStructured -fuzztime=30s
```

設計原則は [`docs/PRINCIPLE.md`](docs/PRINCIPLE.md)、設計上の意思決定は [`docs/ADR`](docs/ADR)（[DocDag](https://github.com/Kaikei-e/DocDag) で検証）に記録されています。

## ライセンス

[Apache License 2.0](LICENSE)
