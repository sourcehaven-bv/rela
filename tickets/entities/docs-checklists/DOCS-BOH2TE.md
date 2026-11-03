---
id: DOCS-BOH2TE
type: docs-checklist
title: 'Docs: lease service (TKT-EIC19E)'
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (Service doc: claim vs renew face rule, version and wake contract, reload limitation of the in-process fallback)
- [x] Function/type docs if public API (Service methods, Gate/Authorizer contracts, appbuild.Leases/WatchLeases)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: internal/lease row added with TKT-6QC5EL)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] ~~API docs updated (if applicable)~~ (N/A: surfaces and their docs land with TKT-27XVYS and TKT-4YPG4Q)
