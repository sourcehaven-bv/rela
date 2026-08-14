---
id: REV-E9DYW5
type: review-checklist
title: 'Review: ICS feed serves `visible:`-redacted properties verbatim'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Provenance of this record

**Backfilled.** BUG-E9DYW5 was completed and merged via
[PR #1314](https://github.com/sourcehaven-bv/rela/pull/1314) without a
`review-checklist` entity, so `done-bug-needs-review-done` was violated. The
violation went undetected because `AM-feed-field-redaction.md` had unparseable
YAML frontmatter (unquoted `visible:` / `where:` in plain scalars), which made
`store.ListEntities` error and silently under-count — `analyze validations`
reported "all 120 rules passed" while skipping the affected entities.

This checklist records **what verifiably happened on that PR**, from the PR's
own review history and merge state. It is not a re-review of the code, and it
does not assert that anyone other than the named reviewer examined it.

## Code Review

- [x] Reviewed and approved before merge

PR #1314 carries 4 review events from **tschmits**, concluding in `APPROVED`,
and was merged 2026-08-14T10:47Z. That approval is the review of record for
this bug.

## Automated Checks

- [x] CI green on the merge commit

The PR merged into `develop`, which requires the full CI suite to pass. No
per-job results are transcribed here — asserting specific local command output
that nobody ran would be fabrication. The authoritative record is the PR's own
check history.

## Acceptance Verification

- [x] Fix pinned by tests that fail without it

Per `AM-feed-field-redaction`, three tests back the fix, all verified to fail
against its removal:

- `TestDeclarativeFeed_RedactsHiddenProperties` — a hidden property never
  reaches a rendered event.
- `TestDeclarativeFeed_RedactionDoesNotChangeMembership` — pins the **order**
  (filter before redact) by hiding the property the `where:` clause filters on;
  redacting first would drop the event and make feed membership vary per
  principal.
- `TestDeclarativeFeed_RedactionCopies` — the shared store entity is not
  mutated.

## Follow-up

The bug's own `prevention` field records the durable fix as **not** done here:
a distinct type for "redacted, safe to render" so a raw `*entity.Entity` cannot
reach a serializer by accident. Redaction is currently enforced per-read-shape,
so each new read path must remember it. That remains tracked separately.
