---
id: RR-SJISFW
type: review-response
title: Removing embedded projections destroys the BUG-TMGWIN safety property and breaks historical step validation
finding: 'The plan removes from_projection/to_projection from migration files and proposes re-basing step validation on (stored projection -> live schema). Not equivalent: validateDeltasResolved validates each file against its OWN historical edge, which is what closed BUG-TMGWIN. Against a whole-chain diff, per-file delta resolution becomes unanswerable and a do-nothing file passes again. Separately, 13 step Validate(from,to) impls check targets against the PRE-migration shape, so historical rename steps cannot validate against a live schema where the old property no longer exists. Recommend keeping the projections and dropping only the hashes, which is all that data-only migrations actually require.'
severity: critical
resolution: 'Accepted option C (Jeroen, 2026-09-19): keep both embedded projections, drop only the from:/to: hashes. The from==to parse refusal at file.go:75-77 exists solely because the hashes exist, so removing them is sufficient to unlock data-only migrations. validateDeltasResolved and all 13 step Validate(from,to) implementations keep their inputs and are unchanged. A data-only migration embeds two projections with no shape delta between them, so no needs-migration delta is raised and no step is demanded. Plan updated: item 3 (remove embedded projections) is struck from scope.'
status: addressed
---

## Finding

The plan removes `from_projection:`/`to_projection:` from migration files and
proposes re-basing step validation on "(stored projection → live schema)". That
substitution is **not equivalent**, and it silently reopens a bug that was
closed on purpose.

### Why the substitution fails

`validateDeltasResolved` (`file.go:165-213`) recomputes the file's own edge and
refuses a file whose needs-migration delta no step answers. Its doc comment
states the property exactly:

> Without this, a migration satisfies the gate by DECLARING the right `to` hash
> while its steps do nothing — which is exactly how BUG-TMGWIN reached
> production. … The deltas are recomputed from the file's OWN embedded
> projections, so the check needs no live schema and **cannot drift from what
> the file claims to do.**

The last clause is the point. Each file is validated against *its own historical
edge*. With embedded projections gone, the only shapes available are the store's
current state and today's live schema — so:

- **An old migration can no longer be validated at all.** File 3 of 7 describes a
transition between two shapes that exist nowhere once the store has moved on.
Validating it against (stored → live) checks a different edge than the one the
file encodes.
- **`resolvingSteps` enforcement collapses.** With a whole-chain diff there is
one aggregate `faces_introduced` delta, not one per file, so "does THIS file
answer its OWN delta" becomes unanswerable. A do-nothing file in the middle of
the chain passes again — precisely BUG-TMGWIN.

### It also breaks ordinary step validation

13 `Validate(from, to)` implementations exist (`steps.go`), and several check
against `from`, the **pre-migration** shape:

```go
// steps.go:137-141
func (s *renamePropertyStep) Validate(from, to metamodel.ShapeProjection) error {
    if !entityPropInShape(from, s.Entity, s.From) {
        return fmt.Errorf("property %s.%s is not in the from-schema", s.Entity, s.From)
    }
```

`rename_property{from: status, to: state}` is valid only where `status` exists
in the from-shape and `state` in the to-shape. Against the live schema `status`
is long gone, so every historical rename step fails validation — or validation
has to be skipped for applied files, which discards the guarantee that step
targets are real rather than silent no-ops (a documented property: "a typo'd
entity type or property is an error, never a silent no-op").

### Severity

Critical. Not a missing edge case — the plan as written would either reintroduce
BUG-TMGWIN or force abandoning step-target validation. Both are regressions in
safety properties that exist because they already failed once in production.

## Options

**A — Keep `from_projection` only; drop `to_projection`.** The from-shape is
what step validation and delta-resolution need; the to-shape is derivable (apply
the declared steps' shape effect) or simply unnecessary if the file no longer
claims an edge identity. Cuts most of the repo-size win while preserving both
safety properties. Files keep one embedded projection, no hashes.

**B — Validate at generation time, record the verdict.** `gen` has both shapes,
so it can run `validateDeltasResolved` then. But an operator hand-writes and
hand-edits these files, so a check that only runs at generation is a check that
does not run.

**C — Keep both projections, drop only the hashes.** Files lose `from:`/`to:`
(so `from == to` stops being a parse error and data-only migrations work), keep
the projections for validation. Smallest change that achieves the ticket's
actual goal. A data-only migration embeds one projection with no shape delta
between them.

## Recommendation

**C**, falling back to **A** if the size cost proves real. The ticket's goal is
data-only migrations and a name-keyed applied-list; neither requires removing
the projections. The hashes are what block `from == to` — the projections are
what make the file self-validating, and the inventory confirms no testdata
fixtures exist to migrate, so keeping them costs nothing in churn.

This narrows the change and removes the single riskiest item from the plan.

## Evidence

- `internal/datamigration/file.go:165-213` — `validateDeltasResolved` + doc
- `internal/datamigration/file.go:136-163` — `resolvingSteps` table
- `internal/datamigration/file.go:112` — `s.Validate(fromProj, toProj)`
- `internal/datamigration/steps.go` — 13 `Validate(from, to)` impls; `:137`,
`:184`, `:230`, `:380`, `:557` read `from`
- `internal/datamigration/migrateface_test.go:280` —
`TestParseFile_RefusesUnmigratedFaceAdoption`, the BUG-TMGWIN regression test
- `internal/datamigration/migrateface_test.go:308` —
`TestResolvingSteps_CoversEveryMigrationDeltaKind` (AM measure)
