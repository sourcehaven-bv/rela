---
id: DOCS-ZRSFEI
type: docs-checklist
title: 'Docs: realtime.yaml loader (TKT-EYCIKU)'
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (anchor read requirement, ceiling per source, reload keep-previous, alias node bound)
- [x] Function/type docs if public API (realtimecfg.Compiled, Role, Gate, Source.NeedsCeilingCheck, AnchorMismatches, realtimelimits.AtMost)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule; realtime.yaml is documented in docs/realtime.md)
- [x] Help text accurate (if CLI changes) (rela acl audit counts realtime.yaml permissions; no new flags)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (docs/realtime.md and GUIDE-realtime describe realtime.yaml, ceilings, limits and reload)
