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
persisted through CI until someone tripped over it by accident. That asymmetry
is the argument for failing closed: the loud one needed no ticket, the quiet one
hid a merged-`done` bug with no review record for days.

## Independently found twice

Two people hit this within days of each other, from different directions, and
both stopped at the symptom rather than the tooling:

- **TKT-1ESTYJ / #1337** quoted the frontmatter as a drive-by fix while working
  on analyze memory, which re-exposed the hidden BUG-E9DYW5 violation. Its
  backfilled `REV-E9DYW5` closes with exactly this diagnosis: *"A parse failure
  during an analyze scan should be an error, not a warning that degrades into an
  under-count — otherwise 'all rules passed' can mean 'the rule never saw the
  file'. Worth its own ticket."* That ticket is this one.
- Separately, while resolving merge conflicts for **#1319** (scheduler failure
  backoff), repairing the same file surfaced the same violation.

Neither encounter was looking for a validation bug; both tripped over it. That
is the argument for fixing the tool rather than the files — the next occurrence
will also be found by accident, if at all.

## Still reproduces

Re-verified on `develop` at 93240281 (after #1337 landed): restoring the
unquoted frontmatter still yields `EXIT=0` and "All validations passed" while
logging 2 `failed to parse frontmatter` errors. The individual files were
repaired; the behaviour that hid them was not.

## Mitigated in CI, not fixed in the tool (#1363)

TKT-W76LRP added an **"Entities all parse"** step to `ci.yml` that runs the
store scan and greps stderr for `failed to parse frontmatter`, failing the job
when it matches. That closes the merge-gate hole this ticket was filed about,
and it is why this is no longer urgent.

It does not fix the bug. Re-verified on `develop` at 4259daca, after #1363 and
#1360 landed: a project containing one unparseable entity still makes

```console
$ rela validate --check cardinality --check properties --check validations
All validations passed.
EXIT=0
```

`rela validate` continues to report success over a corpus it did not fully
read. The CI job now catches it out-of-band by pattern-matching a log line —
which works, but means the guarantee lives in a grep in a YAML file rather than
in the command's exit code. Anyone running `rela validate` locally, or from a
different pipeline, still gets a green result on a partial read.

The remaining work is the one-line-ish version of the fix: propagate the
iterator error to a non-zero exit and name the offending files. The CI step can
then become redundant rather than load-bearing.

## Duplicate

Filed independently as **BUG-RMCK9U** (merged in #1330's stack) while this
ticket was open. That one is closed `wont-fix` pointing here.
