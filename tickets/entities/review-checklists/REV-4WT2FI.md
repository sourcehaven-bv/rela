---
id: REV-4WT2FI
type: review-checklist
title: 'Review: CalDAV: alias service — own injected CalDAV↔rela identity service (rename-safe), not an observer bolt-on'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: the alias
service is internal plumbing — no config key, no CLI flag, no API surface. Its
behaviour is documented as godoc on `caldavalias` and in the deletion-semantics
section of TKT-MF1CWZ.)
- [x] ~~User-facing documentation updated~~ (N/A: nothing user-facing to
document; the alias table is invisible to operators and clients alike.)
- [x] ~~Docs-checklist marked as done~~ (N/A: none created.)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1308 <!-- e.g., https://github.com/org/repo/pull/123 -->

## Evidence (TKT-WAA092 — CalDAV alias service)

Reviewed as part of the CalDAV code review; see **REV-E7QYNN** for the full
finding table and automated-check figures.

Two findings landed here, both linked to this ticket:

| ID | Severity | Status |
|----|----------|--------|
| RR-I4FN1T | significant | addressed — `LookupByEntity` map-order nondeterminism flipped the served href between polls; `Put` now evicts a second href for the same entity and selection is deterministic |
| RR-3UAG12 | significant | deferred — retained aliases grow unbounded and every write rewrites the whole table |

The alias service also gained its central role during this branch: it IS the
deletion tombstone. "Alias exists + entity missing ⇒ deleted ⇒ 404" is inferred
from server state alone, which is what let an earlier client-marker design
(`X-RELA-ENTITY-ID`) be reverted — RFC 5545 §3.8.8.2 lets any client drop an
unknown x-property, so that design failed OPEN.

Per the ticket ACs: own package with its own arch-lint component (OK);
consumers bind narrow interfaces (`AliasRewriter`); a client-created bare-UUID
to-do maps to a new entity and survives restart (verified live); rename rewrites
the alias; **AC5 changed deliberately** — deleting an entity no longer removes
its alias, because that record is the tombstone (`AliasRewriter.EntityDeleted`
doc corrected to match, since it stated the inverse); case-insensitive IDs fold
to one entry; corrupt store fails loudly *at the CalDAV boundary* rather than at
process start (RR-27WGOX).

Coverage 95.7%. `just lint` 0 issues.

**Not done:** PR (`/pr`), shared across the CalDAV tickets.
