# Changelog

このファイルは利用者向けの変更履歴です。日付の無い版は、まだタグを打っていません。

## 0.2.0

まだリリースしていません。タグ `v0.2.0` は [`docs/RELEASING.md`](docs/RELEASING.md) の手順で作ります。

### 追加

- v0.2 スキーマ言語。スキーマは `@lkal "0.1"` 文書で、`lkal-schema = "0.2"` を持つ。
- `github.com/Kaikei-e/LKAL/schema` の `Compile`、`Validate`、`ValidateValue`、`Default`、`Complete`。
- 検証エラー `L501`〜`L510` とスキーマコンパイルエラー `L601`〜`L607`。
- `lkal check --schema`、`lkal set --schema`、`lkal schema check`。
- `Error.Path` と `(*File).Locate`。`lkal version`。

### 変更

- スカラー共用体の `one-of` は、共用体のすべての型を制約できないとき `L606` になる。`.int` と `.decimal` は同じ 10 進の許可集合を共有する。
- 循環する `default` 展開はコンパイル時に `L606` で拒否する。
- `lkal-schema` が `"0.2"` 以外の文字列のときは `L607` だけを返す。
- `Set` で置き換えた値のスキーマエラーは、値に位置が無いときキーの位置を使う。

## 0.1.0

v0.1 の文書構文、正準形、LKAL-JSON、パス操作、`lkal` コマンド。この版のタグは打っていません。
