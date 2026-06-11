---
id: TKT-YGCX
type: ticket
title: Build-tag seams in appbuild + cli/mcp_wiring composition roots
kind: refactor
priority: medium
effort: m
status: done
---

## Description

Introduce per-build helper functions in the two composition roots
(`internal/appbuild` and `internal/cli/mcp_wiring`) so the store backend and
search backend can be swapped at compile time. Default build (`!postgres &&
!memorybackend`) keeps fsstore + bleve; a new `-tags memorybackend` build wires
memstore + LinearSearch.

This is the prerequisite for a future `-tags postgres` companion that wires a
Postgres-backed store + Postgres FTS searcher without dragging bleve/fsstore
into the binary. The memorybackend variant is included now to prove the seam
actually swaps and to give a real second backend.

The user explicitly rejected a bundled `Backend` interface as a re-creation of
the deleted `workspace` god object. The pattern is per-build helper functions
(one file per build per concern), not a runtime registry.

Side effect: `internal/appbuild/testfixture.go` was a non-test file that
imported bleveindex directly, so the production package leaked bleve into every
binary. Extracted into a sibling `internal/appbuild/appbuildtest/` package
mirroring the `internal/store/storetest` pattern. Added an exported
`appbuild.NewFromCollaborators` constructor so the external test package can
assemble `*Services` without poking unexported fields.

## Outcome

Measured on `cmd/rela-server`:

| Build | Binary | Bleve packages |
|-------|--------|----------------|
| default (FS + bleve) | 40 MB | 66 |
| `-tags memorybackend` (memstore + LinearSearch) | 24 MB | 0 |

## Follow-ups

- Collapse `cli/mcp_wiring`'s composition logic into `appbuild` so the
duplicated seams disappear (architect's call from review — the duplication is
the smell, not the seams themselves).
- Move triplicated `backfill` helper into `internal/search` after the
composition roots unify.
- Implement the `postgres` companion files when SaaS deployment lands.
- **Service layering in `appbuild.Collaborators`** (raised in crit review,
  round 2): the bundle mixes infra (`FS`, `Paths`, `Store`, `Searcher`,
  `StateKV`), domain (`Meta`, `Tracer`, `Validator`, `Templater`,
  `CfgLoader`, `ACL`), and orchestrator (`EntityManager`) at the same
  level. Plan: define a read-side facade
  (`Store`+`Meta`+`Tracer`+`Searcher`), make `EntityManager` the
  canonical write surface, narrow `FS`/`Paths`/`StateKV` to per-callsite
  interfaces. Discuss plan with reviewer before any code lands.
