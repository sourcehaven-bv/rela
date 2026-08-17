---
id: REV-E9DYW5
type: review-checklist
title: 'Review: ICS feed serves visible:-redacted properties verbatim'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Why this entity exists

Backfilled by TKT-1ESTYJ, not authored alongside the fix.

BUG-E9DYW5 shipped in **#1314** ("fix(dataentry): redact visible:-hidden
properties in the ICS feed") and was moved to `done` without a linked
`has-review` checklist, which the `done-bug-review-checklist` rule requires.
That violation went unnoticed because the bug's own measure entity,
`AM-feed-field-redaction.md`, carried an unquoted `visible:` in its frontmatter
title and description. The entity therefore failed to parse, so every analyze
scan silently under-counted — `rela validate` reported "all rules passed" while
skipping the file (the run logs a `failed to parse frontmatter` warning that is
easy to miss).

TKT-1ESTYJ quoted that frontmatter as a drive-by fix, which re-exposed the
pre-existing violation. This checklist records what actually happened on #1314;
it does not claim a review this author performed.

## Automated Checks

Per #1314, which merged green through the standard pipeline (test, lint,
arch-lint, coverage, e2e). Not re-run here — the code is unchanged by
TKT-1ESTYJ.

- [x] CI green on #1314 at merge

## Code Review

- [x] **4 GitHub reviews on #1314.** The fix originated in a security review of
      the CalDAV work (#1308), which inherited the same unredacted mapping.

## Acceptance Verification

Verified by the measures the bug itself links, all in
`internal/dataentry/feed_provider_test.go`:

- [x] `TestDeclarativeFeed_RedactsHiddenProperties` — a `visible:`-hidden
      property never reaches a rendered event.
- [x] `TestDeclarativeFeed_RedactionDoesNotChangeMembership` — pins the ORDER,
      by hiding the property the `where:` clause filters on and asserting the
      event still appears. Moving redaction before the filter drops it, so feed
      membership would otherwise vary per principal.
- [x] `TestDeclarativeFeed_RedactionCopies` — the shared store entity is not
      mutated.

All three were verified to fail against the fix being removed (per IMPL-E9DYW5).

## Note

The real lesson is the one this backfill exposes rather than the bug it closes:
a malformed frontmatter field made an entity invisible to validation, and
validation reported success anyway. A parse failure during an analyze scan
should be an error, not a warning that degrades into an under-count — otherwise
"all rules passed" can mean "the rule never saw the file". Worth its own ticket.
