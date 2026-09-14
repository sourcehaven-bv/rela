---
id: DOCS-I51ZDJ
type: docs-checklist
title: 'Documentation: partial cascade delete audit'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — three carry real weight:

  1. The abort-path comment in `deleteEntity` states *why* `DeletedEntities`
stays empty, and now states it in a form that is exactly true. Its first version
claimed the invariant held on "every abort path below", which code review showed
was false for the attachment path (RR-UE2XS7). A comment asserting an invariant
the code violates is worse than no comment; this is the one place a future
reader would trust prose over tracing.
  2. `removeAttachmentDir`'s failure is now non-fatal, which is surprising
enough to need its reason inline — the entity and relations are already gone, so
failing would report failure for an operation that worked.
  3. The `deletedRelations` / `related` index-alignment invariant is documented
**at the producer** ("one append per entry, no skips … Keep it that way"),
because the removal loop's `deletedRelations[i]` is only defensible while it
holds.

- [x] Function/type docs if public API — `store.EntityWriter.DeleteEntity`'s
godoc is the load-bearing one: it is the only place a future backend author
learns that a non-nil result may accompany a non-nil error, that it is **not**
success, and that `DeletedEntities` is populated *if and only if* the entity
file was removed. Review corrected the original wording, which stated only when
the field is empty and so told a caller nothing about when it is populated
(RR-TYG8OV).

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: applies the existing audit and Tx-tier rules
rather than changing them. DEC-8UIL0 already records that fs/mem `Tx` is mutual
exclusion only — this ticket is a consequence of that decision, not a revision
of it.)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no CHANGELOG in this repo; releases are
cut from commit history — `docs/releasing.md`)
- [x] API docs updated — new "Partial cascade delete on a non-transactional
backend" section in `docs-project/entities/guides/GUIDE-audit-log.md`,
regenerated into `docs/audit-log.md` via `./scripts/generate-docs.sh` (generated
file, never edited directly; regeneration touched only it).

Placed under "Known gaps" beside the crash-window entry, because it is the same
family: audit-versus-reality divergence on a non-transactional backend. It
states what an operator will actually see — the removed relations logged under
the usual `cascade:delete-entity:<id>` label, no `delete-entity` row, and that
PostgreSQL/SQLite roll back so the question does not arise there.
