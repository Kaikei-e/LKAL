# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository status

LKAL is a declarative data format with a Go implementation (standard library only). The spec draft v0.1 lives in `docs/internal/` and is gitignored, so it never appears in commits or in agy batch worktrees. No Go code exists yet; the first milestone is the core library defined in `docs/ADR/0001-*.md`.

## Decision records

ADRs live in `docs/ADR/` and are validated with DocDag v0.5 (`docdag.yaml`). Create a record with `docdag new "<english title>" [--depends-on NNNN]`, then write the body in Japanese following `docs/ADR/template.md`. Run `docdag validate` before committing an ADR.

## agy-orch-mcp delegation

The `agy_orch` MCP server (batch + fetch enabled) is configured in the gitignored `.mcp.json`. The workflow is in `docs/ADR/0003-*.md`. In short:

- Claude Code decomposes work into task packets and reviews diffs; agy workers implement.
- `antigravity_batch` requires a clean tree (untracked files count) and workers see only committed files, so paste the relevant spec sections into each packet.
- Batch workers run in temp worktrees outside agy's trusted workspaces: keep them to file edits only and let gates run `go test` / `go vet` / `gofmt`.
- Worker model: `worker: {tier: "reasoning", model: "gemini-3.8-flash-high"}`.
