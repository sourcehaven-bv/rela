---
id: DOCS-STATSQL
type: docs-checklist
title: 'Documentation: runtime state in the database'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — two decisions carry their reasoning:
  why the motivation differs from the postgres backend's (single-process, so
  the problem is being outside the file rather than node-local), and why
  `statesql` does not import `internal/state` (that package wraps it, so
  naming it would invert the dependency).
- [x] Function/type docs if public API — package doc, `KV`, `New`, `Get`,
  `Put`, `Delete` and `backendOverrides` all documented, including why an
  oversize value is rejected rather than truncated.

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-visible surface change — the state is
  read and written by the same services as before)
- [x] ~~CLAUDE.md updated~~ (N/A: deferred with the rest of the feature to
  Phase C, when `db dump`/`db load` make config-and-state-in-the-database an
  operator-facing thing rather than an internal one)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no changelog file in this repo)
- [x] ~~API docs updated~~ (N/A: no HTTP or MCP surface touched)
