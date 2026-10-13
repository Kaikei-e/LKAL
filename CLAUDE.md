# CLAUDE.md

This file provides guidance to host agents (Claude Code or Codex) when working with code in this repository.

## Repository status

LKAL is a declarative data format with a Go implementation (standard library only). The spec draft v0.1 lives in `docs/internal/` and is gitignored, so it never appears in commits or in agy batch worktrees. The foundation scope is implemented: lexer, parser, AST, data model, semantic validation, error reporting, and Equal. Format, production JSON, path operations, and CLI remain outside this milestone.

## Verification commands

- `go test ./...` (about 5 s; `TestParseScalesLinearly` checks that 4x input costs at most 8x time)
- `go test -short ./...` skips the timing check
- `go test -run TestX ./...`
- `go vet ./...`
- `go test -run '^$' -fuzz=FuzzParse -fuzztime=30s -parallel=2`

## Decision records

ADRs live in `docs/ADR/` and are validated with DocDag v0.5 (`docdag.yaml`). Create a record with `docdag new "<english title>" [--depends-on NNNN]`, then write the body in Japanese following `docs/ADR/template.md`. Run `docdag validate` before committing an ADR.

## agy-orch-mcp delegation

The `agy_orch` MCP server (batch + fetch enabled) is configured in the gitignored `.mcp.json`. The workflow is in `docs/ADR/0003-*.md`. In short:

- Host (Claude Code or Codex) is orchestration-only (no Go code editing): decomposes work into task packets and reviews diffs; agy workers implement.
- `antigravity_batch` requires a clean tree (untracked files count) and workers see only committed files. Task packets paste relevant `docs/internal/agy` contracts (`00-common` and `10-api` mandatory), while workers read the gitignored full spec directly from an explicit absolute read-only host-checkout path; never copy or commit the private spec (do not put personal absolute paths in tracked documents).
- Use `antigravity_batch` only for genuinely parallel work. A single task (a RED step, a focused fix) goes through `antigravity_run` in the working tree with `mode: accept-edits`: edits survive a denied command and the same conversation can continue. A batch worker that tries a shell command loses the whole attempt, and the scheduler retries it as RATE_LIMIT / TRANSIENT_INFRA, so watch `events.jsonl` under `~/.local/state/agy-orch/` and stop a batch after two such retries.
- TDD: pin intended behaviour with exact tests first (RED commit), then implement (GREEN commit); no push.
- After GREEN, an Opus review with a differential harness against the previous implementation and an independent oracle caught regressions the suite did not pin; keep doing that for parser changes.
- Review: Opus review can be a Claude subagent or a read-only agy-hosted Opus run.
- Batch workers run in temp worktrees outside agy's trusted workspaces: keep them to file edits only and let gates run `go test` / `go vet` / `gofmt`.
- Worker model: `worker: {tier: "reasoning", model: "gemini-3.8-flash-high"}`.
