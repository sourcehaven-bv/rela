---
id: REV-GEYB
type: review-checklist
title: 'Review: Decouple dataentry from internal/workspace type imports'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] No critical, no significant findings
- [x] 1 minor finding addressed (doc comment no longer names workspace.WatchOptions)
- [x] 1 leverage finding verified-and-rejected (onDataReload IS called from StartWatching)
- [x] Tests pass under `-race`
- [x] `just ci` green

## Disposition

Mechanical type-decouple. Cranky's verdict: "solid, surgical." See IMPL-AC1I for
the table.

**One real fix:** dataentry's doc comment originally named
`workspace.WatchOptions` — exactly the coupling the ticket removes. Now it talks
about the wiring site abstractly.

**Confirmed non-issues:**
- Adapter duplication across server/desktop/test helper is intentional. Extracting a helper would have to live somewhere that re-creates the coupling.
- `storage.ChangeEvent` on dataentry's surface is not a new dependency — `internal/storage` was already imported for FS types.
