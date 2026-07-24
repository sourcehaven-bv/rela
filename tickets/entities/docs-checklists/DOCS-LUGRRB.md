---
id: DOCS-LUGRRB
type: docs-checklist
title: 'Docs: Author-aware version capture (last_edited_by attribution)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported API (`store.Attribution`, `WithAttribution`, `AttributionFrom`) documents the boundary-populated contract, NULL-when-absent rule, and partial-unknown semantics
- [x] Migration 0006 header documents NULL encoding, no-backfill self-healing, and rename re-key neutrality
- [x] `withStoreAttribution` / `attributionValues` / `sweepAttribution` helpers document their invariants and RR references

## Project Documentation

- [x] CLAUDE.md content-versioning bullet updated: the two sanctioned boundary-populated attribution routes (VersionInput + Attribution ctx carrier), the NULL→version-sweep fallback, and the TKT-0IGI4V pointer for author-boundary segmentation

## External Documentation

- [x] `docs/postgres-backend.md` attribution paragraph rewritten: real-editor attribution on swept versions, fallback semantics, self-healing legacy rows, rename neutrality, and the documented v1 last-editor-of-burst limit
- [x] ~~docs/cli-reference.md~~ (N/A: no command changes)
- [x] ~~docs/data-entry.md~~ (N/A: UI unchanged — it simply starts showing real names)
- [x] ~~docs/metamodel.md~~ (N/A)
