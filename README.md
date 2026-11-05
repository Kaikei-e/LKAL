# LKAL

[![CI](https://github.com/Kaikei-e/LKAL/actions/workflows/ci.yml/badge.svg)](https://github.com/Kaikei-e/LKAL/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Kaikei-e/LKAL.svg)](https://pkg.go.dev/github.com/Kaikei-e/LKAL)

[日本語](README.ja.md) · [Design Principles](docs/PRINCIPLE.md) · [ADR](docs/ADR) · [Apache-2.0](LICENSE)

LKAL (Literals, Keys, and Labels) is a minimal data format for declaring desired state. A value's type is decided by its syntax alone, structure is written only with `{}`, and labels compress nested records so every field has a named `/` path.

```lkal
@lkal "0.1"

### Frontend web server
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

This repository holds the Go reference implementation: a parser with semantic validation, the canonical formatter, LKAL-JSON conversion, path operations, and the `lkal` command. It uses only the standard library. The format is at **draft v0.1** and may change.

## Values

| Syntax | Go type | LKAL-JSON |
| --- | --- | --- |
| `"text"`, `\\` lines | `String` (`string`) | `"text"` |
| `42` | `Int` (`int64`) | `42` |
| `0.25` | `Decimal` (normalized base-10 `string`) | `0.25` |
| `true`, `false` | `Bool` (`bool`) | `true`, `false` |
| `.tcp` | `Enum` (`string`, without `.`) | `{"$enum": "tcp"}` |
| `512Mi`, `30s` | `Quantity` (`struct{ Number, Unit string }`) | `{"$qty": "512Mi"}` |
| `&secret/db-creds` | `Ref` (`[]string`) | `{"$ref": "secret/db-creds"}` |
| `["a", "b"]` | `List` (`[]Value`, scalars of one type) | `["a", "b"]` |
| `key { … }`, `key "label" { … }`, `key "label" = …` | `Record` (`struct{ Fields []Field }`, labels expand to nested records) | `{"key": {…}}` |

Every document starts with `@lkal "0.1"` (`L105`, `L207`). Comments start with `###` and multi-line raw strings start with `\\` on every line. Quoted strings allow `\"`, `\\`, `\n`, `\r`, `\t`, and `\u{HEX}` (`L003`); UTF-8 BOM and control characters other than `\t` are rejected (`L008`). Numbers are signed base-10 integers (`int64`) and decimals up to 100 digits (`L005`); leading zeros (`01`), radix prefixes (`0xFF`, `0b10`, `0o77`), exponent notation (`1e5`), and compound units (`1h30m`, `L004`) are rejected. Bare words in value position (except `true` and `false`), `null`, `$`-prefixed keys or labels (`L205`), and duplicate keys or conflicting paths (`L201`) are rejected. Blocks and labels nest at most 1000 levels deep (`L107`). See [`docs/PRINCIPLE.md`](docs/PRINCIPLE.md) for the design rules.

## Install

```sh
go get github.com/Kaikei-e/LKAL
go install github.com/Kaikei-e/LKAL/cmd/lkal@latest
```

Requires Go 1.27.3 or later (uses `encoding/json/jsontext` and `encoding/json/v2` from the standard library).

## Usage

```go
import lkal "github.com/Kaikei-e/LKAL"

f, err := lkal.Parse("app.lkal", src)
if err != nil {
	log.Fatal(err) // *lkal.ErrorList
}

data := f.Data() // lkal.Record
```

`Parse`, `Format`, `FromJSON`, `ParseValue`, `Get`, `Set`, and `Delete` return `*lkal.ErrorList` (`Errors []*lkal.Error`), reporting up to 20 errors sorted by 1-based line and Unicode code point column (`Pos{Line, Col}`). Each error has a stable code and a hint:

```text
bad.lkal:2:12: L102 bare word in value position
  hint: write "three" for a string or .three for an enum
```

Error codes are grouped by stage:

| Range | Stage | Codes |
| --- | --- | --- |
| `L001`–`L009` | Lexical | `###` comments (`L001`, `L002`), escapes (`L003`), compound units (`L004`), `int64` / 100-digit range (`L005`), directive (`L006`), unclosed string (`L007`), invalid char / BOM (`L008`), malformed token (`L009`) |
| `L101`–`L107` | Syntax | broken header (`L101`), bare word value (`L102`), unclosed bracket (`L103`), gap in `\\` lines (`L104`), `@lkal` declaration (`L105`), unexpected token (`L106`), >1000 nesting depth (`L107`) |
| `L201`–`L207` | Semantic | duplicate key or conflicting path (`L201`), mixed-type list (`L204`), `$`-prefixed key or label (`L205`), unsupported version (`L207`) |
| `L301`–`L305` | LKAL-JSON | invalid JSON (`L301`), `null` (`L302`), exponent notation (`L303`), non-scalar list element (`L304`), malformed `$enum` / `$qty` / `$ref` tag (`L305`) |
| `L401`–`L403` | Path / Value | malformed path (`L401`), path not found (`L402`), invalid `Value` for `Set` (`L403`) |

The rest of the API works on a parsed `*lkal.File` or `lkal.Value`:

| Function | Purpose |
| --- | --- |
| `f.Version()`, `f.Data()` | Document version (`"0.1"`) and `Record` data model |
| `lkal.Format(src)`, `f.Format()` | Canonical form (idempotent; keeps comments, blank lines, one-line blocks, and labels) |
| `lkal.ToJSON(f, plain)` | LKAL-JSON (`plain = false`, reversible with `"$lkal"`) or one-way plain JSON (`plain = true`, unwraps `$enum`, `$qty`, `$ref` to strings) |
| `lkal.FromJSON(src)` | Read LKAL-JSON back into a `*lkal.File` |
| `f.Get(path)`, `f.Set(path, v)`, `f.Delete(path)` | Read and edit one value by `/` path, e.g. `service/web/container/app/image`; comments and untouched layout are kept |
| `lkal.ParseValue(src)`, `lkal.FormatValue(v)` | Parse and format a single value (scalars, lists, `\\` lines, or `{ … }` records) as used by `lkal get` and `lkal set` |
| `lkal.Equal(a, b)` | Schema-free semantic equality (record field order is ignored; list order is kept) |

## Command

```text
lkal fmt [-w] [-l] [--json] FILE…
lkal check [--json] FILE…
lkal json [-plain] [--json] FILE
lkal from-json [--json] FILE
lkal get [-r] [--json] FILE PATH
lkal set [--json] FILE PATH VALUE
lkal del [--json] FILE PATH
```

Every command accepts `--json` (or `-json`) to print errors as a single-line JSON array (`[{"file","line","col","code","message","hint"}]`) on stderr. Passing `-` as `FILE` reads from standard input (`set -` and `del -` write the updated canonical LKAL to stdout; `fmt -w -` is an error). `get -r` prints `String`, `Enum`, and `Ref` values in raw form (without quotes, leading `.`, or leading `&`). Exit codes: `0` success, `1` LKAL errors, `2` usage or I/O errors.

```sh
lkal set web.lkal service/web/container/app/image '"ghcr.io/acme/web:1.5.0"'
lkal get web.lkal service/web/container/app/port/http/number   # 8080
lkal get -r web.lkal service/web/container/app/image           # ghcr.io/acme/web:1.5.0
```

## Development

```sh
go test ./...
go test -race -short -count=1 -shuffle=on ./...   # skips timing checks on a heavily loaded machine
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

Design principles are in [`docs/PRINCIPLE.md`](docs/PRINCIPLE.md); architecture decision records are in [`docs/ADR`](docs/ADR) (validated with [DocDag](https://github.com/Kaikei-e/DocDag)).

## License

[Apache License 2.0](LICENSE)
