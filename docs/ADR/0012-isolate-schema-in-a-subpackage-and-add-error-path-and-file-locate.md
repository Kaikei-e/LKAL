---
title: "スキーマ層を schema サブパッケージに分離し、Error.Path と (*File).Locate をコアに追加する"
status: accepted
date: 2026-11-21
depends-on: ["0001", "0002", "0006", "0011"]
---

# 0012: スキーマ層を schema サブパッケージに分離し、Error.Path と (*File).Locate をコアに追加する

## コンテキスト

[[0011]] で定めた v0.2 スキーマ言語と検証器を実装するにあたり、パッケージ境界とエラー位置の解決方法を定める必要がある。

1. **コア層とスキーマ層の依存方向の強制**:
   - `docs/PRINCIPLE.md` は「スキーマが文書のパース・整形・等価判定を変えてはならない」と定めている。[[0001]] では v0.1 のコアライブラリをリポジトリ直下の単一パッケージ `package lkal` に置いたが、スキーマ実装を同じ `package lkal` に入れると、コアの内部構造にスキーマへの依存が紛れ込んでもコンパイラでは検出できない。
2. **データモデル上の論理パスとエラー表示・ソート順**:
   - スキーマ検証エラー（`L501`〜`L510`）や `FromJSON` 由来・`ValidateValue` 由来の値検査では、「どの `/` パスで違反が起きたか」（例: `service/web/replicas`）を機械可読かつ人間にも分かる形で報告する必要がある。また、ソース位置（`Pos.Line > 0`）を持つエラーと位置を持たないエラー（`Pos.Line == 0`）が混在したときの決定論的なソート順が必要である。
3. **構文木からのソース位置の読み取り専用逆引き**:
   - `(*Schema).Validate(f *lkal.File)` はデータモデルを検査しつつ、違反箇所の正確な行・列番号（キーの位置、値の位置、リスト要素の位置、およびラベル圧縮された中間レコードの位置）を報告する必要がある。しかし、`*lkal.File` の構文木ノードは非公開であるため、`schema` サブパッケージから `/` パスに対応するソース位置を逆引きする最小限の公開フックが必要になる。

## 決定

### 1. `github.com/Kaikei-e/LKAL/schema` サブパッケージへの分離

- スキーマのコンパイル、埋め込みメタスキーマ（`meta.lkal`）、データ検証、およびデフォルト補完はすべてサブパッケージ `github.com/Kaikei-e/LKAL/schema`（`package schema`）に配置する（[[0001]] の単一ディレクトリ構成を、スキーマ層に関して精緻化する）。
- 依存方向は `schema` → ルート `lkal`（`github.com/Kaikei-e/LKAL`）の片方向のみとする。Go のパッケージ循環参照禁止により、`lkal.Parse`・`Format`・`Equal`・`ToJSON`・`FromJSON`・`Get`・`Set`・`Delete` がスキーマに一切依存しないことをコンパイル時に保証する。
- `package schema` の公開 API は次のとおりとする。
  - `Compile(name string, src []byte) (*Schema, error)`
  - `CompileFile(f *lkal.File) (*Schema, error)`
  - `(*Schema).ID() string`
  - `(*Schema).Version() string`
  - `(*Schema).Validate(f *lkal.File) error`
  - `(*Schema).ValidateValue(v lkal.Value) error`
  - `(*Schema).Default(path string) (lkal.Value, bool)`
  - `(*Schema).Complete(v lkal.Value) (lkal.Value, error)`

### 2. `lkal.Error` への `Path` フィールド追加とソート規則の拡張

- ルート `package lkal` の `Error` 構造体に `Path string` を追加する。`Path` は [[0006]] の `/` パス規則（`[A-Za-z0-9_.-]` 以外の文字を含むセグメントや空文字列は引用符付き。`true`・`false` は `/` パスでは引用符なし）で整形された論理パスを保持する。
- `(*Error).Error() string` は、`Path != ""` のときに限り、`hint:` 行の直前に `\n  path: <Path>` を出力する。`Path == ""` のときの出力は v0.1 とバイト単位で同一とする。
  ```text
  app.lkal:12:14: L501 expected int, got string
    path: service/web/replicas
    hint: write an integer such as 3
  ```
- `sortErrors` のソート規則を次のとおり拡張する（安定ソートと最大 20 件の上限は維持する）。
  1. ソース位置のあるエラー（`Pos.Line > 0`）を、位置のないエラー（`Pos.Line == 0`）より前に並べる。
  2. 両方とも `Pos.Line > 0` のときは、`Pos.Line` → `Pos.Col` → `Code` → `Path` の辞書順で並べる。
  3. 両方とも `Pos.Line == 0` のときは、`Path`（辞書順）→ `Code` の順で並べる。

### 3. `(*File).Name()` と `(*File).Locate(path []string)` の公開

ルート `package lkal` の `locate.go` に、構文木を一切変更しない読み取り専用のメソッド `(*File).Name() string` と `(*File).Locate(path []string) (Location, bool)`、および `Location` 構造体を公開する。

```go
type Location struct {
	Key   Pos
	Value Pos
	Elems []Pos
}

func (f *File) Name() string
func (f *File) Locate(path []string) (Location, bool)
```

- `(*File).Name()` は `f == nil` のとき `""`、それ以外は `Parse` に渡されたファイル名を返す。
- `(*File).Locate(path)` はセグメント列 `path` に対応する構文木上の位置を返す。
  - `f == nil || f.body == nil` のときは `(Location{}, false)` を返す。
  - `len(path) == 0`（ルート）のときは、`@lkal` 宣言の位置 `f.versionPos` を `Key` と `Value` に入れて `true` を返す。
  - 各エントリのキー＋ラベル列 `spans`（長さ `m`）と残りパス `rem` を比較し、次のように位置を決定する。
    1. **完全一致（`m == len(rem)`）**: `Key` は最後のスパン（`spans[m-1]`）の位置とする。エントリがブロック（`entryRecord`）なら `Value` は `{`（`lbrace`）の位置とする。代入（`entryAssign`）なら `Value` は右辺値の先頭位置とし、右辺が非空のリスト（`*listNode`）であれば各要素の位置を新しいスライス `Elems` に格納して返す。
    2. **`spans` が `rem` の真の接頭辞（`m < len(rem)`）**: エントリがブロックなら `rem[m:]` で子ブロックへ降りる（代入ならその下には進めないため `(Location{}, false)`）。
    3. **`rem` が `spans` の真の接頭辞（`len(rem) < m`）**: ラベル圧縮（例: `service "web" "http" { ... }` に対するパス `["service", "web"]`）によって暗黙に導入された中間レコードの最初の出現位置として、`Key: spans[len(rem)-1].pos`, `Value: spans[len(rem)].pos` を返す。

## 根拠（調査結果・出典）

- [[0001]]（2 層構成）および `docs/PRINCIPLE.md`（Out of Scope: Schema-dependent semantics）:
  - 構文木（`File`）とデータモデル（`Value`）の 2 層を持つルートパッケージから `schema` を別パッケージに切り出すことで、「コアはスキーマを知らない」という設計原則が Go のパッケージグラフそのものによって機械的に検証される。
- [[0002]]（エラーコードと位置）および [[0006]]（`/` パス操作）:
  - ラベル圧縮された項目（例: `service "web" = 1` や `service "web" { ... }`）はデータモデル上では多段の `Record` になる（[[0010]]）。`Locate` がラベル列の途中接頭辞と完全一致の両方を解決することで、どのようなラベル圧縮スタイルで書かれた文書に対しても、スキーマ検証エラーが正確なトークン位置（[[0002]] の 1 始まり行・Unicode コードポイント列）を指すことができる。
  - また、`ValidateValue` や `FromJSON` 由来の `*File` のようにソース位置を持たない場合（`Pos.Line == 0`）でも、`Error.Path` と `Pos.Line == 0` 用の辞書順ソートにより、マップの走査順に依存しない決定論的なエラー出力が得られる。

## 検討した代替案

- **スキーマ検証器をルート `package lkal` に同居させる案**:
  - 非公開の構文木フィールドに直接アクセスできる利点はあるが、ルートパッケージの API と責務が肥大化し、コア関数がスキーマに依存しないことをパッケージ境界で保証できなくなるため不採用とした。
- **構文木の内部ノード（`blockNode`, `entryNode` など）をそのまま公開する案**:
  - 構文木の内部表現を公開すると、将来のフォーマッタや `Set` の改善で構文木を変更できなくなる。パスから位置への射影のみを返す `Locate(path []string) (Location, bool)` を公開すれば、構文木のカプセル化を保ったまま `schema` パッケージに必要な位置情報をすべて提供できる。
- **`Error.Msg` の文字列の中にパスを埋め込む案**:
  - エラーメッセージの文言とデータパスが混ざると、CLI の `--json` 出力やプログラムからの検査でパスを構造化データとして取り出せなくなる。独立したフィールド `Error.Path` とした。

## 影響とトレードオフ

- 得るもの: ルートパッケージの最小性とスキーマ非依存性をコンパイル時に保証しつつ、`schema` パッケージが構文木の正確な行・列番号と `/` パスの両方を持つエラーを報告できる。v0.1 のエラー出力（`Path == ""`）との完全な後方互換性が保たれる。
- 失うもの: スキーマ機能を使う利用者は `github.com/Kaikei-e/LKAL` に加えて `github.com/Kaikei-e/LKAL/schema` を import する必要がある。
- リスク: `Error.Path` と `Location` / `Locate` / `Name` はルートパッケージの公開 API となるため、今後の版でもシグネチャを維持する必要がある。
- 今後: [[0013]] で `cmd/lkal` から `schema` パッケージを呼び出し、`--json` 出力に `path` を統合する。

## 関連ADR

[[0001]], [[0002]], [[0006]], [[0011]], [[0013]]。
