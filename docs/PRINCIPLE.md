# Design Principles

LKAL is a data format for declaring desired state. It is designed to stay small enough to parse with one-token lookahead and zero dependencies, while expressive enough to represent, validate, and edit complex resource manifests without edge cases.

Every rule in the language and library comes from the principles below.

## 1. Types Live in Values

A value's type is decided by its first one or two characters, without looking at a schema or surrounding tokens.

| Form | Type |
| --- | --- |
| `"80"`, `\\` lines | `String` |
| `80` | `Int` |
| `0.25` | `Decimal` |
| `true`, `false` | `Bool` |
| `.tcp` | `Enum` |
| `512Mi`, `30s` | `Quantity` |
| `&secret/db-creds` | `Ref` |
| `["a", "b"]` | `List` |
| `{ … }` | `Record` |

`"80"` is always a string and `80` is always an integer. Strings never turn into booleans or numbers based on spelling.

## 2. Structure Is Only `{}`

Blocks are delimited by `{` and `}`. Indentation has no syntactic meaning; the formatter adds it for readability. Copy-pasting or programmatic edits cannot break structure through whitespace changes.

## 3. Never Guess

Ambiguous input is rejected with a position, a stable error code, and a `hint` instead of being coerced.

- Bare words are not allowed in value position (except `true` and `false`).
- There is no `null`; omit the key when a value is absent.
- Numbers have one notation: base-10 integers and decimals. Hexadecimal (`0xFF`), scientific notation (`1e5`), and digit separators (`1_000`) are rejected (`L009`).
- Duplicate keys and conflicting paths are always errors (`L201`), never overwritten or merged.

## 4. Labels Compress Nested Records

Lists hold only scalars of one type (`L204`). Instead of lists of objects, repeated items are written with labels:

```lkal
service "web" {
  port "http" { number = 8080, protocol = .tcp }
  port "metrics" { number = 9090, protocol = .tcp }
  env "LOG_LEVEL" = "info"
}
```

Labels are syntactic sugar for nested `Record`s (`Entry = Key { Label } ( Assign | Block )`). The block above produces the exact same `Record` as:

```lkal
service {
  web {
    port {
      http { number = 8080, protocol = .tcp }
      metrics { number = 9090, protocol = .tcp }
    }
    env { LOG_LEVEL = "info" }
  }
}
```

This keeps the composite data model to two types (`List` and `Record`) while keeping deep manifests flat and making every field addressable by a named path (`service/web/port/http/number`) rather than an array index.

## 5. Every Line Is Self-Contained

Constructs that span multiple lines mark each line at the start:

- Comments start with `###` on every line. There are no block comments or trailing end-of-line comments.
- Multiline strings start with `\\` on every line.

A lexer or line-based tool can classify any line without reading earlier lines, and comments attach predictably to the entry that follows them.

## 6. Stable Formatting, Schema-Free Equality

Formatting and semantic equality are separate:

- `Format` is idempotent (`Format(Format(x)) == Format(x)`) and does not wrap lines by width. It keeps comments, blank lines, entry order, single-line blocks, and labeled entry syntax.
- `Equal(a, b)` compares data models only. `Record` field order does not matter; `List` element order does. `Equal` never depends on an external schema.

## 7. Closed JSON and Path Operations

The syntax tree, data model, LKAL-JSON, and path edits (`Get`, `Set`, `Delete`) are closed over all valid documents:

- `ToJSON(f, false)` and `FromJSON` round-trip all nine value types without loss of data model information, using four reserved `$` keys (`$lkal`, `$enum`, `$qty`, `$ref`). User keys and labels starting with `$` are rejected (`L205`).
- `Set` and `Delete` edit the syntax tree at a `/` path while keeping comments and layout on untouched entries. Adding a multi-segment path to an empty block writes a labeled entry; deleting the last entry leaves a valid block that `Set` can populate again.

## Out of Scope

LKAL leaves out features that break static reading, path addressing, or deterministic in-place edits:

- **Computation**: variables, expressions, conditionals, loops, string interpolation, and templates.
- **Multi-file or implicit composition**: includes, anchors, inheritance, and implicit overrides.
- **Positional object lists**: lists contain scalars only.
- **Schema-dependent semantics**: a schema (planned for v0.2) may validate values, but it must never change how a document is parsed, formatted, or compared with `Equal`.

## Checklist for Changes

A change to the language or library must satisfy all of the following:

1. Lexing stays line-local and parsing stays `LL(1)` (one-token lookahead).
2. The data model stays within the nine `Value` types.
3. Types and `Equal` depend only on the document text, never on a schema.
4. `Format(Format(x)) == Format(x)` and `Equal(FromJSON(ToJSON(f, false)).Data(), f.Data())` hold for every valid `File`.
5. `Get`, `Set`, and `Delete` work on any path in any valid `File`, including empty blocks and stdin/stdout streams (`-`).
