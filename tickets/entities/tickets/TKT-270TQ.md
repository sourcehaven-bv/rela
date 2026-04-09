---
id: TKT-270TQ
type: ticket
title: Push markdown imports behind repository boundary
kind: refactor
priority: medium
effort: m
status: in-progress
---

## Problem

Consumer packages (`cli`, `dataentry`, `mcp`) directly import
`internal/markdown` to access parsing, formatting, content validation, and
template types. This couples consumers to the on-disk serialization format and
prevents introducing alternative storage backends (zip, WebDAV) without changing
every consumer.

See `.ignored/database-lessons.md` proposal #3 ("Store Interface — Repository as
the Backend Contract") for full context.

## Current violations

| Package | File | Usage |
|---|---|---|
| `cli` | `rename.go:182` | `markdown.NewFileIO(fs).UpdateEntityTypesInDir()` |
| `cli` | `normalize.go:61` | `markdown.NormalizeHeaders()` |
| `dataentry` | `app.go:618` | `markdown.EntityTemplate` type |
| `dataentry` | `analyze.go:426` | `markdown.CheckContentRule()` |
| `mcp` | `tools_helpers.go:187` | `markdown.CheckContentRule()` |

Additionally, `repository.Store` returns `*markdown.Document` and
`[]*markdown.EntityTemplate` from template methods, leaking the markdown type
into consumers.

## Scope

**In scope:**

1. Move `CheckContentRule` usage from `mcp` and `dataentry` behind workspace or validation
2. Move `NormalizeHeaders` usage from `cli` behind workspace
3. Move `UpdateEntityTypesInDir` usage from `cli` behind workspace
4. Introduce model-level types to replace `markdown.EntityTemplate` in the Store interface
5. Remove `markdown` from `cli`, `dataentry`, and `mcp` in `.go-arch-lint.yml`
6. Verify arch lint passes with the new restrictions

**Out of scope:**

- Extracting a full `Store` interface (separate ticket)
- Alternative backends (zip, WebDAV)
- Moving workspace's own markdown usage behind repository
- Snapshot API (#1 from database-lessons.md)

## Acceptance Criteria

1. `internal/cli` has zero imports of `internal/markdown`
2. `internal/dataentry` has zero imports of `internal/markdown`
3. `internal/mcp` has zero imports of `internal/markdown`
4. `.go-arch-lint.yml` forbids `markdown` from `cli`, `dataentry`, and `mcp`
5. `go-arch-lint check` passes
6. `go test -race ./...`, `just lint`, `just coverage-check` all pass
7. No behavioral changes — all existing tests pass without modification
