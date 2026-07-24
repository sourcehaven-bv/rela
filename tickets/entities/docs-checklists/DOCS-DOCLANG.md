---
id: DOCS-DOCLANG
type: docs-checklist
title: 'Documentation: rela-docs phase 2 doc language (TKT-3RLZR4)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package godoc on `internal/mermaid` (injection-safety rationale) and `internal/docs` (island model: statement vs echo)
- [x] Godoc on the exported surface — `docs.Build`, `docs.Options`, `mermaid.StateDiagram`/`Graph`/`Label`, `BuildError`
- [x] Inline comments for the non-obvious logic — the two-graph/raw-store seed, the `dr.pending` typed-error stash, the direction-vs-relation-type filter split, mermaid entity-escaping

## Project Documentation

- [x] User-facing guide added — `docs-project/entities/guides/GUIDE-rela-docs.md` (source) → `docs/rela-docs.md` via `just docs`: island syntax, the resolver reference, the seed/memstore model, `graph` grains + exclude/only, fail-loud + `--strict`
- [x] Example manual committed — `prototypes/data-entry/manual/tickets-manual.md` (builds against the phase-1 prototype project; doubles as living documentation)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting convention; the doc language is a self-contained subsystem documented in its own guide + package godoc)

## External Documentation

- [x] CLI reference — the `rela docs build <manual> [--out] [--strict]` command is documented in the new guide; `docs-project` guide pipeline covers the CLI surface
- [x] ~~API reference~~ (N/A: no wire/HTTP API change — this is a CLI + library feature)
