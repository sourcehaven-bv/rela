---
id: RR-ZYU2GC
type: review-response
title: Reset state enumeration is incomplete — 6 refs missed, several cause silent wrong-data writes
finding: Reset list omits stagedVisibleProps/stagedAffordancesReady, fieldAffordances/relationAffordances, hiddenPolicy's retained map, formGeneration and linkParams. Retention of these leaks record N into record N+1 — the hiddenPolicy one restores a hidden record-1 value over a cleared field on record 2 (silent wrong-data write).
severity: critical
status: addressed
resolution: >-
  Plan step 1 rewritten against the component with all six missed refs (stagedVisibleProps, stagedAffordancesReady, fieldAffordances/relationAffordances, hiddenPolicy.releaseAll, formGeneration, linkParams). Added a pure blankCreateState() extraction so the value half is a return type rather than an audit.
---

## Finding

The plan's reset list omits create-mode reactive state in `DynamicForm.vue`
whose retention leaks record N into record N+1:

- `stagedVisibleProps` (`:186`) / `stagedAffordancesReady` (`:184`) — retain
record 1's dry-run verdicts; interacts with `userTouched` clearing so record 2's
payload can depend on a stale affordance snapshot via
`visibleWritablePropertiesForCommit()` (`:437-458`).
- `fieldAffordances` (`:107`) / `relationAffordances` (`:116`) — on the
`refreshStagedAffordances` fail-open path (`:797-802`) record 1's per-field
readonly/option verdicts persist silently.
- `hiddenPolicy` retained map — `loadEntity` calls `releaseAll()` (`:499`)
precisely because "retained values belong to the form state we are about to
replace". A reset is the same hazard: a value retained while hidden on record 1
is restored over a cleared field when revealed on record 2.
- `formGeneration` (`:1308`) — its own comment says "bumped whenever the form's
underlying entity state is replaced wholesale". A reset is exactly that; without
a bump a `clear_when_hidden: confirm` dialog opened before the submit resolves
against the new record.
- `linkParams` (`:102`) — never reset to null. Idempotent here, but the
`as === 'from'` auto-link `createRelation` (`:1146-1158`) re-runs per record,
which the plan never states.

## Resolution

Rewrite the reset enumeration against the component. Adopt the reviewer's
leverage suggestion: extract `blankCreateState(...)` returning `{ formData,
relations, content }` as plain data so "did we miss a field?" is answerable by
reading one return type instead of auditing 2459 lines, and so the carry-over
logic is unit-testable without mounting.

Verified against source before accepting.
