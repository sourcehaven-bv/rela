---
id: BUG-4KPN2M
type: bug
title: A malformed entity file makes analyze silently under-count, so `rela validate` reports success on rules it never evaluated
description: "collectEntities and FindOrphansWithScope log a warning and return a PARTIAL result when the store iterator errors. `rela validate` then reports 'All validations passed.' and exits 0 for rules that never saw the skipped entity, so a single unparseable frontmatter field hides every violation on that entity from CI."
priority: high
effort: s
status: backlog
why1: "`rela validate` printed 'All validations passed.' and exited 0 while a done bug (BUG-E9DYW5) sat in violation of a required-relation rule."
why2: The entity carrying the violation was never evaluated — its own measure file failed to parse, so it was skipped.
why3: collectEntities logs the iterator error and RETURNS THE PARTIAL SLICE; validation then runs over the surviving entities and finds nothing wrong.
why4: The analysis service has no error channel to its callers, so a read failure can only be a warning — documented in FindOrphansWithScope as a known follow-up, never closed.
why5: "The success message asserts a stronger claim than the code can support: 'all rules passed' is reported for a run that could not read all the input. Silent incompleteness reads as success, so nothing ever surfaced it."
---

## Symptom

A single malformed entity file makes `rela validate` report success while
skipping every rule that would have evaluated that entity.

Found for real: `AM-feed-field-redaction.md` (shipped in #1314) carried an
unquoted `visible:` inside its frontmatter title and description. That entity
therefore failed to parse and vanished from every scan. `rela validate` — the
same command CI runs — printed:

```
Running custom validations...
✓ All validation rules passed

All validations passed.
```

and exited **0**, while `BUG-E9DYW5` was `done` with no `has-review`
relation, violating `done-bug-review-checklist`. Quoting the frontmatter (in
TKT-1ESTYJ) re-exposed the violation immediately.

The only signal was a `WARN` on stderr, easy to miss and not reflected in the
exit code:

```
level=WARN msg="analysis: store.ListEntities iterator error; results may under-count"
  type="" error="failed to parse frontmatter: yaml: line 4: mapping values are not allowed in this context"
```

## Root cause

`internal/analysis/analysis.go`. The shared collector swallows iterator
errors into a partial result:

```go
func collectEntities(ctx context.Context, s store.Store, q store.EntityQuery) []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range s.ListEntities(ctx, q) {
		if err != nil {
			slog.Warn("analysis: store.ListEntities iterator error; results may under-count",
				"type", q.Type, "error", err)
			return out          // <-- partial result, indistinguishable from a complete one
		}
		out = append(out, e)
	}
	return out
}
```

It feeds **nine** call sites (duplicates, gaps, cardinality in both
directions, properties, validations, …), and `FindOrphansWithScope` has the
same shape with two more warn-and-skip paths.

`internal/cli/validate.go` then decides the exit code purely from the
findings it was handed — it has no way to learn the input was truncated —
so an incomplete run and a clean run are the same observable outcome.

This is **known and documented**, not an oversight:

> Errors from the tracer or per-entity store reads are logged via `slog.Warn`
> and the impacted entries are skipped — the caller sees an under-count rather
> than a hard failure. […] A returning-errors variant is a candidate follow-up;
> not in scope for the package lift.

The judgement was defensible for a CLI summary. It is not defensible now that
the same code path is a **merge gate**: the `Rela Tickets` CI job runs
`rela validate --check cardinality --check properties --check validations`
and merges on its exit code.

## Impact

Any unparseable entity file silently narrows what CI checks, and reports the
narrowed run as a pass. The failure is worst exactly when it matters most:
the malformed file is usually *new* work, so the rules most likely to be
skipped are the ones guarding the change being merged.

Note this is the same defect class TKT-1ESTYJ fixed on the memory side —
silent incompleteness presented as a complete result. There, a capped section
reports `truncatedChecks` so the UI can say the list is partial. Here, a
truncated scan reports nothing at all.

## Scope

**In scope**

- Give the analysis service an error channel so a read failure reaches its
  caller instead of dissolving into a warning.
- Make `rela validate` **fail closed**: a scan that could not read all input
  must not print "All validations passed." or exit 0.
- Keep `rela analyze`'s human-facing summary usable — it may still render
  partial results, but it must say so, and its exit code must reflect that
  the run was incomplete.

**Out of scope**

- Making the *store* reject malformed files at load. Tolerating hand-edited
  markdown is deliberate (see the data-entry validation policy); the bug is
  that analysis conceals what it skipped, not that a bad file can exist.
- Auto-repairing frontmatter.

## Acceptance criteria

1. With one unparseable entity file present, `rela validate` exits **non-zero**
   and names the file it could not read.
2. It must not print "All validations passed." on such a run.
3. A run with no read failures behaves exactly as today — same output, same
   exit code (no false positives on healthy projects).
4. `rela analyze` surfaces the incomplete-scan condition rather than silently
   under-counting; partial output is allowed, silent partial output is not.
5. The nine `collectEntities` call sites and the two `FindOrphansWithScope`
   warn-and-skip paths all propagate, with no remaining swallow.

## Test plan

- **Regression test (must fail on current `develop`)**: a fixture project with
  one malformed frontmatter file plus one genuine rule violation on a *different*
  entity; assert `validate` exits non-zero. On today's code the violation is
  reported, so the sharper variant is the real pin: put the violation on the
  entity that *follows* the malformed file in scan order and assert it is still
  caught.
- Assert the specific case that occurred: an entity whose own linked measure is
  malformed must still be evaluated by relation-cardinality rules.
- Healthy-project test asserting exit 0 and byte-identical output, so
  fail-closed does not become fail-noisy.
- Per-call-site coverage that an injected iterator error propagates rather than
  yielding a short slice.

## Notes

Discovered while fixing CI on TKT-1ESTYJ (PR #1337). The YAML quoting for
`AM-feed-field-redaction.md` and the backfilled `REV-E9DYW5` checklist landed
there; this ticket is the underlying defect that let the violation hide.

`internal/dataentry`'s own analyze path has the same `break`-on-error shape in
its section scans. It is less severe (the web UI is not a merge gate) but is
the same pattern and should be considered together.
