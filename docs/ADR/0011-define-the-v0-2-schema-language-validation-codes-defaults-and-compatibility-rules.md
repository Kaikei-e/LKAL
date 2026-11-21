---
title: "v0.2 スキーマ言語、検証・コンパイルエラー L501〜L510・L601〜L607、宣言的デフォルト、および互換性不変条件を定める"
status: accepted
date: 2026-11-21
depends-on: ["0001", "0002", "0004", "0006", "0009", "0010"]
---

# 0011: v0.2 スキーマ言語、検証・コンパイルエラー L501〜L510・L601〜L607、宣言的デフォルト、および互換性不変条件を定める

## コンテキスト

基礎から v0.1 までの実装（[[0001]]〜[[0010]]）により、LKAL 文書の字句・構文・意味規則の検査、正準形整形、LKAL-JSON 往復変換、および `/` パス操作が確定した。`docs/PRINCIPLE.md`（原則 1・3・4・6）は、値の型が構文だけで決まること、ラベルが入れ子の `Record` に展開されること（[[0010]]）、およびスキーマが `Parse`・`Format`・`Equal` の意味論を一切変更してはならないことを不変条件として定めている。

一方で、実用上のマニフェスト管理では「どのキーが必須か」「どの列挙値や単位・参照先を許すか」「ラベル付きレコード（動的キーのマップ）がどの構造を持つか」を静的に検査し、位置・パス・修正ヒント付きのエラーとして報告するスキーマ層（v0.2）が必要になる。スキーマ言語を設計するにあたり、次の点を決定する必要がある。

1. スキーマ自体の記述形式と、データ文書とスキーマの結合方法。
2. 固定フィールドを持つ構造体（`.record`）と任意ラベルを持つ辞書（`.map`）の型指定、単一型・スカラー共用体・名前付き定義参照（`use`）の書き分け、および自己記述メタスキーマ（`meta.lkal`）での表現可能性。
3. 9 種の値型ごとの制約条件と、フィールド属性（`required`, `default`, `doc`, `deprecated`, `ext`）の意味論。
4. 宣言的デフォルト値がコア操作（`Parse`, `Format`, `Equal`, `ToJSON`, `Validate`）に与える影響の境界。
5. データ検証エラーとスキーマコンパイルエラーのエラーコード体系、およびスキーマ変更時の後方互換性の定義。

## 決定

### 1. スキーマ文書の形式と外部バインディング

- スキーマは通常の `@lkal "0.1"` 文書として記述し、トップレベルに `lkal-schema = "0.2"`（必須）、任意の `id`（`String`）・`version`（`String`）・`doc`（`String`）、必須の `root { ... }`、および任意の `def "<name>" { ... }` を持つ。
- データ文書は `@lkal "0.1"` のままとし、`@use` や `@schema` のような文書内ディレクティブは導入しない。データ文書とスキーマの結合は CLI の `--schema` フラグまたは Go API による外部指定のみとし、すべてのデータ文書・スキーマ文書が v0.1 パーサーと完全に互換であることを保証する。

### 2. 型指定（Type Spec）と `meta.lkal` の自己記述構造

- すべての型指定レコードは、次の 3 形式のうち**ちょうど 1 つ**を持たなければならない（欠落は `L601`、複数指定は `L606`）。
  1. `type = .<kind>`: 単一型。`<kind>` は `.string`, `.int`, `.decimal`, `.bool`, `.enum`, `.quantity`, `.ref`, `.list`, `.record`, `.map`, `.any` のいずれか。
  2. `types = [.<scalar1>, .<scalar2>, ...]`: スカラー共用体（Scalar Union）。`.string`, `.int`, `.decimal`, `.bool`, `.enum`, `.quantity`, `.ref` の 7 種のスカラー型から **2 個以上の互いに異なる型**を列挙する。複合型（`.list`, `.record`, `.map`）や `.any` は `types` に含められない。
  3. `use = &def/<name>`: トップレベルの `def "<name>"` への参照（2 セグメント固定の `Ref`）。`use` を指定した型指定レコードには、`type`・`types` およびいかなる型制約キーも併記できない（併記した場合は `L606`。`doc` およびフィールド位置での `required`, `default`, `deprecated`, `ext` のみ可）。
- トップレベルの `root` ブロックは暗黙に `type = .record` として扱い、`doc`, `open`, `reserved`, `field` のみを受け付ける。
- `use` のみによる直接・間接のエイリアス循環（例: `def "a" { use = &def/b }` と `def "b" { use = &def/a }`）はスキーマコンパイル時に `L603` として拒否する。一方、`record` の `field` や `map` の `value` を経由する構造的再帰は許容する（データ文書の深さは [[0009]] の `L107` により最大 1000 段で必ず停止する）。
- `.map` の値型指定 `value` は、`list` の `items { ... }` や `record` の `field "<name>" { ... }` と同様、常に `Record` 型の型指定ブロック（例: `value { use = &def/port }` または `value { type = .string }`）とする。単一型 `type` とスカラー共用体 `types` を分離し、`value` を `Record` に統一することで、自己記述メタスキーマ `meta.lkal` において `type`・`types`・`value` 上の複合型共用体の抜け道（`.any`）を完全に排除できる（`meta.lkal` で `.any` を用いるのは、任意の型を取りうる `default` と、コンパイラ側で列挙値リストとして検証するオープンな `values` のみに限定する）。

### 3. 型ごとの制約条件

各型に指定できる制約キーを次のとおり定める（該当しない型への制約指定は `L601`、上限・下限の逆転など矛盾する制約は `L606`）。

| 型 | 制約キーと意味 |
| --- | --- |
| `.string` | `min-length`, `max-length`（`Int`、0 以上の Unicode コードポイント数）、`pattern`（`String`、Go 標準 `regexp` RE2 による文字列全体一致 `\A(?:...)\z`、不正な正規表現は `L604`）、`one-of`（空でない `String` のリスト） |
| `.int` | `min`, `max`（`Int`、閉区間）、`one-of`（空でない `Int` のリスト） |
| `.decimal` | `min`, `max`（`Int` または `Decimal`、浮動小数点を経由しない正確な 10 進比較）、`one-of`（空でない `Decimal` のリスト） |
| `.bool` | なし |
| `.enum` | `values`（**必須**。空でない `Enum` のリスト） |
| `.quantity` | `units`（空でない ASCII 英字 `[A-Za-z]+` の `String` リスト） |
| `.ref` | `targets`（空でない `Ref` のリスト、セグメント単位の前方一致）、`min-segments`（`Int`、0 以上）、`max-segments`（`Int`、1 以上） |
| `.list` | `items { ... }`（**必須**。スカラーまたはスカラー共用体の型指定）、`min-items`, `max-items`（`Int`、0 以上）、`unique`（`Bool`、`lkal.Equal` による要素の重複禁止） |
| `.record` | `field "<name>" { ... }`、`open`（`Bool`、既定値 `false`。`false` のとき未定義フィールドを `L503` で拒否）、`reserved`（廃止済みフィールド名の `String` リスト。同一レコードで `field "x"` と `reserved = ["x"]` が共存する場合は `L606`） |
| `.map` | `value { ... }`（**必須**。エントリ値の型指定 `Record`）、`key { doc, min-length, max-length, pattern, one-of }`（マップキー文字列への制約）、`min-entries`, `max-entries`（`Int`、0 以上）、`unique-by`（エントリがレコードのとき、全エントリ間で値が重複してはならないフィールド名の空でない `String` リスト。フィールド名の重複は禁止され `meta.lkal` で `unique = true` を課す） |
| `.any` | なし（任意の有効な LKAL 値を受理する） |

スカラー共用体 `types = [...]` では、列挙された各スカラー型に対応する制約キーを併記でき、入力値の実際のスカラー型に対応する制約を適用する（`.enum` を含む場合は `values` を必須とし、共用体に含まれない型の制約キーは `L601` とする）。

### 4. ドキュメント注釈・フィールド属性と宣言的デフォルト

`doc`（`String`）は共通のドキュメント注釈キーであり、トップレベル、`root`、型指定ブロック（`def` / `items` / `value` の `type-spec`）、`key`（`key-spec`）、および `field`（`field-spec`）のすべてで指定できる。

`record` の `field "<name>" { ... }` には、型指定および `doc` に加えて次のフィールド専用キーを指定できる。

- `required`（`Bool`、既定値 `false`）: フィールドが必須であることを示す。
- `default`（任意の LKAL 値）: フィールド省略時の宣言的デフォルト値。スキーマコンパイル時に、そのフィールド自身の型指定と制約を満たすことを検査し、違反があれば `L605` を報告する。`required = true` と `default` の同時指定は矛盾するため `L606` で拒否する。
- `deprecated`（`String`）: 非推奨理由の注釈文字列。警告専用システム（`L590` 等）は設けず、検証の合否には影響しないメタデータとする。
- `ext`（オープンな `Record`）: 外部ツール用の拡張メタデータ領域。スキーマ検証器は中身を無視する。

**宣言的デフォルトの不変条件**:
- `Parse`、`Validate`、`Format`、`Equal`、`ToJSON` は、スキーマの `default` 値を暗黙に注入したり補完したりしない。
- デフォルト値の参照と具体化は、`(*Schema).Default(path string) (lkal.Value, bool)` と、入力値を変更せず新しい `lkal.Value` を返す純粋関数 `(*Schema).Complete(v lkal.Value) (lkal.Value, error)` のみを通じて明示的に行う。
- `Complete` は、親の `Record` がすでに存在する場合にのみ、その中の省略された任意フィールドに `default` を埋める（存在しない親レコードを勝手に生成しない）。`Complete` は冪等（`Complete(Complete(v)) == Complete(v)`）であり、構文木（`*lkal.File`）を直接書き換える `Fill` のような破壊的 API は設けない。

### 5. エラーコード（`L501`〜`L510` および `L601`〜`L607`）

スキーマ検証およびスキーマコンパイルでは、途中で打ち切らずにすべての違反を収集し、ソートして最大 20 件まで `*lkal.ErrorList` として返す。

#### データ検証エラー（`L501`〜`L510`）

| コード | メッセージの趣旨 | 発生条件と hint |
| --- | --- | --- |
| `L501` | type mismatch | 値の型が `type` または `types` と一致しない。期待される型を hint に示す |
| `L502` | missing required field | `required = true` のフィールドが欠落している。追加すべきフィールド名を hint に示す |
| `L503` | unknown field | `open = false` のレコードに未定義のフィールドがある、または `reserved` に含まれるフィールドが使われた。`reserved` の場合は廃止済みである旨を、それ以外は編集距離（Levenshtein 距離）2 以内の候補があれば `did you mean "<candidate>"?`、なければ許可フィールド一覧を hint に示す |
| `L504` | value out of range | 数値の `min`/`max`、文字列の `min-length`/`max-length`、参照の `min-segments`/`max-segments`、リストの `min-items`/`max-items`、マップの `min-entries`/`max-entries` の範囲外 |
| `L505` | value does not match pattern | 文字列が `pattern` の正規表現に全体一致しない |
| `L506` | value not allowed | 値が `one-of` または `.enum` の `values` に含まれない |
| `L507` | unit not allowed | `Quantity` の単位が `units` に含まれない |
| `L508` | reference target not allowed | `Ref` が `targets` のいずれの接頭辞にも一致しない |
| `L509` | duplicate value | `.list` の `unique = true`、または `.map` の `unique-by` で指定されたフィールド値が重複している |
| `L510` | invalid map key | `.map` のエントリキーが `key { ... }` の制約（長さ・パターン・`one-of`）に違反している |

#### スキーマコンパイルエラー（`L601`〜`L607`）

| コード | メッセージの趣旨 | 発生条件 |
| --- | --- | --- |
| `L601` | invalid schema | `meta.lkal` 違反、未知のキー、型指定（`type`/`types`/`use`）の欠落、必須制約（`values`/`items`/`value`）の欠落、型と無関係な制約キー、スカラー以外の `list.items` 等 |
| `L602` | unresolved definition | `use = &def/<name>` の参照先 `<name>` が `def` に存在しない |
| `L603` | definition cycle | `use` の連鎖が `record` や `map` を介さずに循環している |
| `L604` | invalid pattern | `pattern` の文字列が RE2 正規表現としてコンパイルできない |
| `L605` | default does not match its type | `default` の値がそのフィールドの型指定・制約に違反している |
| `L606` | conflicting constraints | `min > max`、`min-length > max-length`、`min-segments > max-segments`（または `max-segments < 1`）、`min-items > max-items`、`min-entries > max-entries`、`required = true` と `default` の同時指定、`field` と `reserved` の名前重複、`type` と `types` の同時指定、`use` と他キー（`type`/`types`/型制約）の併用、`unique-by` の対象が `.record` でないまたは閉じた `.record` に未定義のフィールド、空のリスト制約（`values`/`one-of`/`units`/`targets`/`unique-by`）、`one-of` と他の制約の矛盾など |
| `L607` | unsupported schema version | `lkal-schema` が `"0.2"` でない |

### 6. スキーマ互換性の不変条件

- スキーマの変更が**後方互換（backward-compatible）**であるとは、「旧スキーマの下で有効であったすべての LKAL 文書が新スキーマの下でも有効であり、かつ既存フィールドの `default` 値の意味論が変わらないこと」と定義する。
- したがって、必須フィールドの追加、任意フィールドから必須への変更、型や制約の縮小（`min` の引き上げ、`max` の引き下げ、`values` / `one-of` / `units` / `targets` の削減）、`open = true` から `false` への変更、フィールドの削除・改名（`reserved` への移動を含む）、および既存の `default` 値の変更はすべて**破壊的変更（breaking change）**となる。
- `schema export` や `schema diff` といったサブコマンドは v0.2 の範囲外とする。

## 根拠（調査結果・出典）

- `docs/PRINCIPLE.md` 原則 1（Types Live in Values）および原則 6（Stable Formatting, Schema-Free Equality）:
  - 値の型と意味的等価性（`lkal.Equal`）は文書テキストのみから決まる。もしスキーマの `default` を `Parse` や `Equal` に暗黙注入すると、スキーマの有無で `Equal(a, b)` や `ToJSON(f, false)` の結果が変化し、[[0010]] の決定 4（`lkal.Equal` の純粋関数性）に違反する。デフォルト補完を `(*Schema).Complete` に隔離することで、コアの不変条件を完全に維持できる。
- [[0010]]（`Collection` の `Record` への統合）:
  - LKAL のデータモデルでは、`service "web" { ... }` のようなラベル付き項目は入れ子の `Record` に展開される。スキーマ層で固定キーの `.record`（`field "<name>"`）と動的ラベルの `.map`（`key` と `value`）を同じ `Record` データモデルに対する 2 種類の検証規則として定義することで、データモデルの 9 型（[[0010]]）を一切増やさずに固定構造とラベル付き辞書構造の双方を検証できる。
- [[0001]]（標準ライブラリのみによる実装）および [[0009]]（入れ子上限 1000 段、`L107`）:
  - 正規表現に Go 標準の `regexp`（RE2）を採用することで、外部依存ゼロと線形時間のマッチングを保証できる。また、構造的再帰を許してもデータ文書側が [[0009]] の 1000 段上限を持つため、検証器の再帰も有界時間で必ず停止する。

## 検討した代替案

- **データ文書内に `@schema "path.lkal"` ディレクティブを導入する案**:
  - 文書自体がファイルシステム上の相対パスや外部 URI に依存するようになり、`Parse` の純粋性と v0.1 パーサーとの後方互換性が損なわれるため不採用とした。
- **`map` の `value` に参照（`value = &def/x`）や型名（`value = .string`）の直接代入を許す案**:
  - `value` が `Record`・`Ref`・`Enum` の複数型を取りうることになり、複合型共用体を持たないスキーマ言語自身の規則で `meta.lkal` の `value` フィールドを記述できなくなる（`.any` に逃げる必要が生じる）。`value { ... }` を常に `Record` に統一すれば、`meta.lkal` が完全な自己記述性を獲得できるため不採用とした。
- **構文木（`*lkal.File`）にデフォルト値を直接書き込む `(*Schema).Fill(f *lkal.File)` を設ける案**:
  - ユーザーが書いていないフィールドを構文木に挿入すると、コメント位置や記述順の保持という構文木の責務（[[0005]], [[0006]]）と衝突する。データモデル値（`lkal.Value`）に対する不変な `Complete` のみを提供する形を採用した。
- **`deprecated` フィールドの使用時に警告コード（`L590` 等）を出力する案**:
  - LKAL のエラーモデルは「成功（エラー 0 件）」か「失敗（エラー 1 件以上、CLI 終了コード 1）」の二値であり（[[0002]], [[0007]]）、警告レベルを導入すると CLI や `--json` を利用する自動化ループの判定が複雑化する。`deprecated` は宣言的メタデータにとどめる。

## 影響とトレードオフ

- 得るもの: v0.1 の文法・9 種のデータモデル・JSON 往復・`Equal` の純粋性を一切変更せずに、9 型すべてに対する制約検査、自己記述メタスキーマ `meta.lkal`、宣言的デフォルト補完、および編集距離ヒント付きのエラー（`L501`〜`L510`, `L601`〜`L607`）を提供できる。
- 失うもの: データ文書単体を見てもどのスキーマに従うかは分からず、CLI や呼び出し側で `--schema` を明示する必要がある。また、リスト要素はスカラーまたはスカラー共用体に限られる（複合要素の繰り返しには原則 4 のとおりラベルと `.map` を使う）。
- リスク: `L501`〜`L510` および `L601`〜`L607` は公開後に変更できない安定コードとなる。
- 今後: パッケージ構成と構文木の位置逆引き（`Locate`）を [[0012]] で、CLI のサブコマンドと `--schema` 連携を [[0013]] で定める。

## 関連ADR

[[0001]], [[0002]], [[0004]], [[0006]], [[0009]], [[0010]], [[0012]], [[0013]]。
