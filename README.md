# LKAL

> LKAL (Literals, Keys, and Labels) is a minimal data format for declaring desired state.

```
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

This repository holds the Go reference implementation: a parser with semantic validation, the canonical formatter, LKAL-JSON conversion, path operations, and the `lkal` command. It uses only the standard library.

## Status

The format is at draft v0.1 and may change.

## Install

```sh
go get github.com/Kaikei-e/LKAL
go install github.com/Kaikei-e/LKAL/cmd/lkal@latest
```

Requires Go 1.27 or later (LKAL-JSON uses `encoding/json/jsontext`).

## Usage

```go
import lkal "github.com/Kaikei-e/LKAL"

f, err := lkal.Parse("app.lkal", src)
if err != nil {
	log.Fatal(err) // *lkal.ErrorList
}

data := f.Data() // lkal.Record
```

`Parse` reports up to 20 errors, sorted by position. Each error has a stable code and a hint.

```
bad.lkal:2:12: L102 bare word in value position
  hint: write "three" for a string or .three for an enum
```

The rest of the API works on a parsed `*lkal.File`:

| Function | Purpose |
| --- | --- |
| `lkal.Format(src)`, `f.Format()` | Canonical form (idempotent; keeps comments, blank lines and one-line blocks) |
| `lkal.ToJSON(f, plain)` | LKAL-JSON (`plain = false`, reversible) or plain JSON |
| `lkal.FromJSON(src)` | Read LKAL-JSON back into a `*lkal.File` |
| `f.Get(path)`, `f.Set(path, v)`, `f.Delete(path)` | Read and edit one value by path, e.g. `service/web/container/app/image`; comments are kept |
| `lkal.ParseValue(src)`, `lkal.FormatValue(v)` | Value text as used by `lkal get` and `lkal set` |

Blocks nest at most 1000 levels deep (`L107`).

## Command

```
lkal fmt [-w] [-l] FILE…
lkal check FILE…
lkal json [-plain] FILE
lkal from-json FILE
lkal get [-r] FILE PATH
lkal set FILE PATH VALUE
lkal del FILE PATH
```

Every command accepts `-json` to print errors as a JSON array on stderr. Passing `-` as `FILE` reads from standard input (`set -` and `del -` write the updated canonical LKAL to stdout; `fmt -w -` is an error). `get -r` prints scalar values in raw form (without quotes, leading `.`, or leading `&`). Exit codes: 0 success, 1 LKAL errors, 2 usage or I/O errors.

```sh
lkal set web.lkal service/web/container/app/image '"ghcr.io/acme/web:1.5.0"'
lkal get web.lkal service/web/container/app/port/http/number   # 8080
lkal get -r web.lkal service/web/container/app/image           # ghcr.io/acme/web:1.5.0
```

## Values

| Syntax | Go type |
| --- | --- |
| `"text"`, `\\` lines | `String` |
| `42` | `Int` |
| `0.25` | `Decimal` |
| `true` | `Bool` |
| `.tcp` | `Enum` |
| `512Mi`, `30s` | `Quantity` |
| `&secret/db-creds` | `Ref` |
| `["a", "b"]` | `List` (scalars of one type) |
| `key { … }`, `key "label" { … }`, `key "label" = …` | `Record` (labels expand to nested records) |

`lkal.Equal` compares two values. Record fields match in any order; lists match in order.

## Development

```sh
go test ./...
go vet ./...
go test -run '^$' -fuzz=FuzzParse -fuzztime=30s
go test -run '^$' -fuzz=FuzzFormat -fuzztime=30s
go test -run '^$' -fuzz=FuzzFromJSON -fuzztime=30s
```

`go test -short ./...` skips the timing checks, which can fail on a heavily loaded machine.

Design principles are in [`docs/PRINCIPLE.md`](docs/PRINCIPLE.md); design decisions are recorded in [`docs/ADR`](docs/ADR).

## License

[Apache License 2.0](LICENSE)
