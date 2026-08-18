---
id: DOCS-0L7R4O
type: docs-checklist
title: 'Docs: Cut MCP peak memory 24x: persistent on-disk search index reused across restarts'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

New exported API on `bleveindex.Index` — `DocCount`, `SetWatermark`, `Watermark`
— each carries godoc saying what it is for, not just what it does.
`SetWatermark` documents that it stores the value verbatim with no MAX
semantics, specifically to distinguish it from `LastModified`, which does.

The three genuinely non-obvious decisions are documented at the point where
someone would otherwise undo them:

- **`NewMem` godoc** states that an empty-path scorch index starts neither the
persister nor the merger (`scorch.go:311`), so nothing merges per-write segments
and heap grows with (writes x distinct docs) — with the measured 5.7GB figure
and a pointer to `New`. This is the trap that nearly shipped; the note exists so
the next person does not rediscover it in production.
- **`New` godoc** explains the bbolt lock and why `bolt_timeout` is set
(the default is an unbounded block, which would hang startup rather than
degrade), and records that `unsafe_batch` was measured and rejected because
`Close` stops the persister instead of draining it.
- **`indexIsCurrent` godoc** explains that the store's `LastModified` (newest
file mtime on disk) and the index's `LastModified` (entity `UpdatedAt`, set in
memory) are *different clocks* several milliseconds apart, so comparing them
reports "stale" on an index that is current. That mistake was made and fixed
during implementation; the comment is what stops it recurring.

`backfillChunkSize` documents why chunking exists, and the error-path `break`
documents that continuing would leave the chunk undrained — the critical finding
from RR-3JA0ZZ.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: introduces no new
architectural pattern or rule — it changes where an existing derived index
lives, and `.rela/` already hosts derived state such as the audit log, caldav
aliases and the state KV)
- [x] ~~docs/ updated for changed behaviour~~ (N/A: see below)
- [x] ~~Architecture docs updated~~ (N/A: no package boundary, dependency or
wiring-contract change; `arch-lint` clean)

## External Documentation

- [x] ~~README updated~~ (N/A: no user-visible feature)
- [x] ~~CLI reference updated~~ (N/A: no new or changed command or flag)
- [x] ~~API docs updated~~ (N/A: no HTTP/MCP surface change)

## Rationale for N/A

No user-facing surface changes: no CLI flag, config key, HTTP or MCP endpoint,
schema or metamodel change. Observable behaviour is identical apart from
resource usage — same search results, same order, verified against the baseline
binary through the real MCP tool.

Two operator-visible consequences were weighed and judged not to need doc
changes:

1. **A new `.rela/search` directory appears.** `.rela/` is already gitignored
and already contains derived, disposable state; nothing documents its contents
file-by-file, so there is no list to keep in sync. The index is rebuilt
automatically if deleted or corrupted.
2. **A second concurrent process logs a fallback warning.** The warning text
is self-explanatory and names the path; it is the diagnostic, so documenting it
elsewhere would only add a second place to go stale.

If a future change makes the index location or the reuse behaviour configurable,
that is the point at which `docs/` needs an entry.
