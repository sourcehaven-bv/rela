---
id: DOCS-5SWO3OL
type: docs-checklist
title: 'Docs: TKT-LTSNHD'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why each hop is gated before the next, why pairs are DISTINCT, why the path stops at three relations
- [x] Function/type docs if public API: godoc on `RollupDef.Via`, `MaxRollupPath`, `rollup.Hop`, `store.RollupStep` and `RollupQuery.Path`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-metamodel.md` roll-up section: `via` as a path, a paths paragraph with an example, far-end `where`, distinct counting, the limit of three, and per-hop access; regenerated `docs/metamodel.md` with `scripts/generate-docs.lua`
- [x] ~~Security guide~~ (N/A: the read gate is the existing per-type read query, applied per hop; no new ACL concept)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API shape change; roll-up values keep their wire shape)
