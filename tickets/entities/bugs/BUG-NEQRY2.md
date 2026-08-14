---
id: BUG-NEQRY2
type: bug
title: rela validate exits 0 when entity frontmatter fails to parse — validation silently skips those entities
description: A single entity file with unparseable YAML frontmatter makes store.ListEntities return an iterator error. rela validate logs it as a WARN, skips the entity, and still prints 'All validations passed' with exit 0. CI is therefore green on a corpus it never fully read, and any rule violation on the skipped entity is invisible.
priority: high
effort: s
status: backlog
---

## Symptom

One entity file with unparseable YAML frontmatter is enough to make `rela
validate` report success while **not having validated that entity**.

Reproduced against this repo's own `tickets/` project by restoring the exact
breakage that shipped in PR #1314:

```console
$ cd tickets && ../bin/rela validate \
    --check cardinality --check properties --check validations
...
All validations passed.
EXIT=0
```

— while the same run emitted **2** `failed to parse frontmatter` errors. The
affected entity was skipped, not validated.

## Why this matters

It is not a cosmetic logging issue. The skipped entity's rule violations
disappear with it, so a green CI run can hide real failures.

That is exactly what happened:

- `AM-feed-field-redaction.md` had unquoted `visible:` / `where:` inside plain
scalars (`title:` / `description:`), so its frontmatter failed to parse.
- The parse failure made `store.ListEntities` error and under-count.
- `done-bug-needs-review-done` therefore never evaluated **BUG-E9DYW5**, which
had been merged `done` with no review checklist at all.
- CI on `develop` reported "All 120 validation rules passed" throughout.

The violation only surfaced when the frontmatter was repaired — i.e. the
corpus-level failure was *masking* an item-level failure, and the masking was
silent in both directions.

## Root cause (initial reading, to confirm)

`store.ListEntities` surfaces the problem as an **iterator error** rather than a
hard failure. Callers log it at WARN (`analysis: store.ListEntities iterator
error; results may under-count`) and carry on with a partial result set. For an
*analysis* command "best effort over what parsed" is arguable; for a
**validation gate whose exit code gates merges**, a partial read is not a pass.

## Proposed fix

`rela validate` should fail closed: if any entity could not be read or parsed,
exit non-zero and name the offending files, regardless of whether the entities
that *did* parse are clean. A validation run that skipped input has not
validated the project.

Worth deciding separately whether other read paths should also harden or keep
degrading gracefully — `validate` is the one with a merge gate behind it, so it
is the one that must not lie.

Consider also a cheap dedicated check (`rela validate --check frontmatter`, or
folding it into the existing checks) so the failure names the file and the YAML
error directly rather than surfacing as a downstream under-count.

## Prior art in this repo

This is the **second** occurrence of the same YAML class, both from an unquoted
`key:` inside a plain scalar:

- `AM-date-property-write-roundtrip.md` — `due: 2026-08-12` in `description:`.
Blocked *all* entity creation (`collect existing IDs: failed to parse
frontmatter`), so that one at least failed loudly.
- `AM-feed-field-redaction.md` — `visible:` / `where:`. Did **not** fail
loudly; it silently disabled validation for the affected entities.

The first was noisy and got fixed immediately; the second was silent and
persisted through CI. That asymmetry is the argument for failing closed.

## Provenance

Found while resolving merge conflicts for PR #1319 (scheduler failure backoff),
after repairing the second broken file exposed the hidden BUG-E9DYW5 violation.
