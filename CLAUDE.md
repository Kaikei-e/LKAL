# CLAUDE.md

This file provides guidance to host agents (Claude Code or Codex) when working with code in this repository.

## Repository status

LKAL is a declarative data format with a Go implementation (standard library only, Go 1.27.3 for `encoding/json/jsontext`). The spec draft v0.1 lives in `docs/internal/` and is gitignored, so it never appears in commits. Implemented: lexer, parser, AST, data model, semantic validation, error reporting, Equal (ADR 0001–0002), LKAL-JSON (0004, 0008), the canonical form (0005), path operations (0006), the `lkal` command (0007), the 1000-level nesting limit (0009), collection-as-record and number/CLI hardening (0010), and v0.2 schema validation in `github.com/Kaikei-e/LKAL/schema` (0011–0013: compile `L601`–`L607`, validate `L501`–`L510`, `Complete`/`Default`, `lkal check --schema`, `lkal set --schema`, `lkal schema check`). Out of scope for v0.1: `Unmarshal` with `lkal:"name"` tags. Per-document schemas are the v0.2 `schema` package, not an in-document directive.

## Verification commands

- `go test ./...` (about 10 s; `TestParseScalesLinearly` and `TestFromJSONManyErrorsLinear` check that 4x input costs at most 8x time; under a heavy machine load (load average above about 5) they can fail on any Go version, so rerun them before suspecting the code)
- `go test -short ./...` skips the timing checks
- `go test -run TestX ./...`
- `go vet ./...`
- `go test -run '^$' -fuzz=FuzzParse -fuzztime=30s -parallel=2` (also `FuzzFormat`, `FuzzFromJSON`, `FuzzEqual`, `FuzzParseValue`, `FuzzPath`, `FuzzStructured`, and in `./schema` `FuzzCompile`, `FuzzValidate`)

## Decision records

ADRs live in `docs/ADR/` and are validated with DocDag v0.5 (`docdag.yaml`). Create a record with `docdag new "<english title>" [--depends-on NNNN]`, then write the body in Japanese following `docs/ADR/template.md`. Run `docdag validate` before committing an ADR.

- Ground ADR rationale (`## 根拠（調査結果・出典）`) in LKAL's own invariants, formal properties, and measurements. Do not casually cite external products or languages as corroboration.
- If prior art is genuinely referenced (such as Zig in the spec draft), verify its true origin, cite the primary specification URL, represent its semantics accurately, and respect license / clean-room boundaries.

## Workflow

- TDD: pin intended behaviour with exact tests first (RED commit), then implement (GREEN commit); no push.
- After GREEN, an Opus review with a differential harness against the previous implementation and an independent oracle caught regressions the suite did not pin; keep doing that for parser changes.
- Review: run reviews as Claude Code Opus subagents (Agent tool, `model: "opus"`). They may run probes in the scratchpad and should verify each finding by running code.
- Never copy or commit the private spec in `docs/internal/`, and do not put personal absolute paths in tracked documents.
