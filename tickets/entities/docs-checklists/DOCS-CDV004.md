---
id: DOCS-CDV004
type: docs-checklist
title: 'Docs: CalDAV alias service'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package godoc on `internal/caldavalias` — why an injected service rather than a store observer, and why the alias is keyed by `(collection, href)`
- [x] Documented that the alias table doubles as a TOMBSTONE: "alias exists + entity missing ⇒ deleted ⇒ 404", an inference drawn entirely from server-side state so it holds regardless of client behaviour (RFC 5545 §3.8.8.2 lets any client drop an X- property, so a marker-property design fails open)
- [x] Documented why the alias is KEPT on a soft delete — dropping it makes the next PUT read as a create and resurrect the entity
- [x] `AliasRewriter.EntityDeleted` godoc corrected: it notifies, it does not drop

## Project Documentation

- [x] ~~User-facing docs~~ (N/A: the alias service is internal plumbing — no config key, no CLI flag, no API surface. Operators and clients never see it; its behaviour surfaces only as correct deletion semantics, which `docs/caldav.md` documents)

## External Documentation

- [x] ~~README / external docs~~ (N/A: internal package)

**Docs verified:** the deletion-semantics table (alias present + entity deleted
→ stable 404; no alias + unseen href → 201) is recorded on TKT-MF1CWZ and was
verified live, including against an out-of-band `rm`.
