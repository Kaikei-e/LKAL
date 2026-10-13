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

This repository holds the Go reference implementation: a lexer, a parser, and a data model with semantic validation. It uses only the standard library.

## Status

The format is at draft v0.1 and may change. A formatter, JSON conversion, path operations and a CLI are not implemented yet.

## Install

```sh
go get github.com/Kaikei-e/LKAL
```

Requires Go 1.26 or later.

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
| `key { … }` | `Record` |
| `key "label" { … }` | `Collection` |

`lkal.Equal` compares two values. Record fields and collection elements match in any order; lists match in order.

## Development

```sh
go test ./...
go vet ./...
go test -run '^$' -fuzz=FuzzParse -fuzztime=30s
```

Design decisions are recorded in [`docs/ADR`](docs/ADR).

## License

[Apache License 2.0](LICENSE)
