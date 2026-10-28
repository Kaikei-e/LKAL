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
| `"text"`, `\\` lines | `String` | `"text"` |
| `42` | `Int` (`int64`) | `42` |
| `0.25` | `Decimal` (exact base-10 `string`) | `0.25` |
| `true`, `false` | `Bool` | `true`, `false` |
| `.tcp` | `Enum` | `{"$enum": "tcp"}` |
| `512Mi`, `30s` | `Quantity` (`{Number, Unit}`) | `{"$qty": "512Mi"}` |
| `&secret/db-creds` | `Ref` (`[]string`) | `{"$ref": "secret/db-creds"}` |
| `["a", "b"]` | `List` (scalars of one type) | `["a", "b"]` |
| `key { … }`, `key "label" { … }`, `key "label" = …` | `Record` (labels expand to nested records) | `{"key": {…}}` |

Comments start with `###` and multi-line strings start with `\\` on every line. Bare words in value position (except `true` and `false`), `null`, radix prefixes (`0xFF`), exponent notation (`1e5`), `$`-prefixed keys, and duplicate keys or conflicting paths are rejected. Blocks and labels nest at most 1000 levels deep (`L107`). See [`docs/PRINCIPLE.md`](docs/PRINCIPLE.md) for the design rules.

## Install

```sh
go get github.com/Kaikei-e/LKAL
go install github.com/Kaikei-e/LKAL/cmd/lkal@latest
```

Requires Go 1.27.3 or later (LKAL-JSON uses `encoding/json/jsontext`).

## Usage

```go
import lkal "github.com/Kaikei-e/LKAL"

f, err := lkal.Parse("app.lkal", src)
if err != nil {
	log.Fatal(err) // *lkal.ErrorList
}

data := f.Data() // lkal.Record
```

`Parse` reports up to 20 errors, sorted by position. Each error has a stable code and a hint:

```text
bad.lkal:2:12: L102 bare word in value position
  hint: write "three" for a string or .three for an enum
```

The rest of the API works on a parsed `*lkal.File` or `lkal.Value`:

| Function | Purpose |
| --- | --- |
| `f.Version()`, `f.Data()` | Document version (`"0.1"`) and `Record` data model |
| `lkal.Format(src)`, `f.Format()` | Canonical form (idempotent; keeps comments, blank lines, one-line blocks, and labels) |
| `lkal.ToJSON(f, plain)` | LKAL-JSON (`plain = false`, reversible with `"$lkal"`) or one-way plain JSON |
| `lkal.FromJSON(src)` | Read LKAL-JSON back into a `*lkal.File` |
| `f.Get(path)`, `f.Set(path, v)`, `f.Delete(path)` | Read and edit one value by `/` path, e.g. `service/web/container/app/image`; comments are kept |
| `lkal.ParseValue(src)`, `lkal.FormatValue(v)` | Value text as used by `lkal get` and `lkal set` |
| `lkal.Equal(a, b)` | Schema-free semantic equality (record field order is ignored; list order is kept) |

## Command

```text
lkal fmt [-w] [-l] FILE…
lkal check FILE…
lkal json [-plain] FILE
lkal from-json FILE
lkal get [-r] FILE PATH
lkal set FILE PATH VALUE
lkal del FILE PATH
```

Every command accepts `-json` to print errors as a JSON array on stderr. Passing `-` as `FILE` reads from standard input (`set -` and `del -` write the updated canonical LKAL to stdout; `fmt -w -` is an error). `get -r` prints scalar values in raw form (without quotes, leading `.`, or leading `&`). Exit codes: `0` success, `1` LKAL errors, `2` usage or I/O errors.

```sh
lkal set web.lkal service/web/container/app/image '"ghcr.io/acme/web:1.5.0"'
lkal get web.lkal service/web/container/app/port/http/number   # 8080
lkal get -r web.lkal service/web/container/app/image           # ghcr.io/acme/web:1.5.0
```

## Development

```sh
go test ./...
go test -short ./...   # skips timing checks on a heavily loaded machine
go vet ./...
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
