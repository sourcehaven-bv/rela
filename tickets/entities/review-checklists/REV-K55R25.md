---
id: REV-K55R25
type: review-checklist
title: 'Review: Actions: gate entity_id on the read path, fix the writeMu DoS'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./internal/dataentry/` — pass
- [x] `go vet` — clean
- [x] `gofmt` — clean
- [x] ~~`just lint` locally~~ (OOM-killed in the worktree, signal 9 — an
environment limit, not a finding; CI runs the full linter)

## Code Review

Run with the cranky-code-reviewer agent. It **rejected the first attempt**, and
was right on every count: the fix closed the row-level hole and opened two
others. The reviewer built the commit in a worktree, wrote probe tests, ran
them, then applied its proposed alternative and re-ran everything — so the
findings are demonstrated, not asserted.

| Finding | Severity | Status |
|---|---|---|
| `visible:`-redacted field values still reached script scope | critical | fixed |
| Reintroduced the existence oracle it set out to prevent | critical | fixed |
| Denial broke actions that never touch `entity` | significant | fixed |
| The right seam (`visibility.ScriptReader`) already existed | significant | adopted |
| `stored = nil` control flow reads as three states | minor | fixed |
| `entity_type` now unused and undocumented | minor | fixed |
| Actions have no WRITE authorization at all | significant | out of scope → TKT-YH52OM |

### What I got wrong

**I hand-rolled a gate at the call site when the package already shipped the
seam.** `visibility.ScriptReader.GetEntity` does all three things I needed —
stored-type gating, field redaction, and `store.ErrNotFound` on denial — and its
godoc reads almost as a description of the bug. I checked `gateRead` was
available on `writeHandler` and stopped looking.

**My commit message asserted a property the code did not have.** It claimed
`visible:`-hidden fields no longer reached script scope. `gateRead` is
`PermitsRead` — row-level only — and `EntityToTable` redacts nothing, so a
principal permitted the row still read hidden field values out of the action's
own response. A false security claim is worse than none: the next person greps
for the fix and stops looking. Corrected in the replacement commit.

**I considered one arm of the oracle and not the other.** I deliberately made
the *absent* branch fall through rather than 404 — and then let the *denied*
branch write `gateRead`'s 404. Denied-vs-absent became a clean binary probe over
the whole id space. The two arms of one decision have to be indistinguishable
from *each other*, which is the RR-NGMI invariant.

**I broke a legitimate caller.** The SPA sends `entity_id` for every selected
list row. Refusing the whole request meant an action whose script ignores
`entity` failed because one row in a selection was invisible to the caller.
`entity_id` is an optional *parameter*, not the resource.

## Mutation testing

Reverting the resolution to the raw store fails **four of seven** tests:

- `TestAction_EntityIDRespectsReadGate`
- `TestAction_EntityIDCrossTypeEscalation`
- `TestAction_HiddenFieldRedacted`
- `TestAction_DeniedAndAbsentAreIndistinguishable`

The redaction test initially passed against broken code — it was wired against
the ACL policy's `Visible:` rather than `app.fieldResolver`, which is the seam
redaction actually resolves through. Rewired to match the `_views` redaction
tests, then confirmed it fails under mutation. A guard that cannot fail is worse
than no guard.

## Verified by the reviewer, not just claimed

- **`set:` actions are safe.** `Action.Set` is consumed only by config
validation; a `set:`-only action reaching `handleV1Action` has `Script == ""`
and dies in `openLocalScript`. Declarative mutation goes through PATCH, gated at
`write_handler.go`.
- **The webhook path needs no equivalent fix.** `dispatchWebhookAction` passes
literal `nil` as `triggerEntity` and builds params from verified JWT claims
only. No caller-supplied id, nothing to gate. Explicitly: do not "align" it by
adding an `entity_id` parameter.
- **Lookup-then-gate ordering is correct.** The stored type must be known to
gate correctly; the entity goes nowhere on deny; both paths do the same store
round-trip so there is no timing differential. `ScriptReader` has the same
shape.

## Acceptance Verification

| Property | Evidence |
|---|---|
| Row-level ACL enforced | `RespectsReadGate` — no-role principal gets nothing |
| Cross-type escalation blocked | `CrossTypeEscalation` — claimed type cannot reach a denied id |
| Field-level redaction enforced | `HiddenFieldRedacted` — `visible:`-hidden value absent from script scope |
| No existence oracle | `DeniedAndAbsentAreIndistinguishable` — same status, same body |
| Availability preserved | `RunsWhenEntityDeniedButUnused` — action runs, 200 |
| `entity_type` inert | `EntityTypeIsIgnored` — three claims, identical responses |
| Permitted path intact | `EntityIDPermittedStillWorks` |
| writeMu DoS closed | resolution happens before `writeMu.Lock()` |

## Follow-up

**TKT-YH52OM** carries the remaining gap the reviewer flagged: actions have no
write authorization either — a principal with read-only grants can create
entities through an action script. That is the capability-gating work, already
filed.
