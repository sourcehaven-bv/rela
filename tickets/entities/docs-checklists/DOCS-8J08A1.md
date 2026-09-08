---
id: DOCS-8J08A1
type: docs-checklist
title: 'Documentation: config.Loader List and layered loader'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious — the two non-obvious choices carry
their reasoning: why `List` stats before `ReadDir` (OsFS and MemFS disagree on
not-a-directory), and why the union copies rather than appends (the primary's
backing array belongs to the layer that returned it).
- [x] Function/type docs if public API — `Loader.List`, `NewLayered`,
`layered.Load`, `layered.List` and `layered.Subscribe` all documented, including
the fail-closed union decision and why the decorator forwards `Subscribe`.

## Project Documentation

- [x] ~~README updated~~ (N/A: `internal/config` is not a user-facing surface)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern — this widens an existing
seam whose package doc already anticipated additional backends. The pattern
entry belongs with Phase C, when `db dump`/`db load` make the mode visible.)
- [x] ~~Help text accurate~~ (N/A: no CLI changes in this ticket)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: internal seam with no behaviour change —
`FSLoader` is the only implementation, so nothing an operator can observe
differs. The user-visible change lands in Phase C.)
- [x] ~~API docs updated~~ (N/A: no HTTP or MCP surface touched)
