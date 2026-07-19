---
id: TKT-3G93B8
type: ticket
title: 'Machine-aware status control: surface _transitions on the wire + SPA performable-transition UI + entry-locked create field'
kind: enhancement
priority: medium
effort: l
status: done
---

<!-- @managed: claude-workflow v1 -->

## Problem

The transition primitive (TKT-E4LW2) and the resolved read query (TKT-FT8J9,
`affordances.PolicyResolver.TransitionVerdicts`) exist but are **dormant** — no
wire surface, no UI. The data-entry SPA still renders a state-machine `status`
field as a plain enum `<select>` listing **all** values, with no notion of which
moves are legal from the current state or which the user may perform. A user can
pick any value; illegal/guarded ones fail on write with a 422/403 toast.

Goal: a **machine-aware status control** — the Linear/Jira pattern — that shows
only the *performable* transitions for this user on this entity, driven by the
`TransitionVerdicts` data that already exists.

## Scope

**In (three coupled pieces):**

1. **Wire surface.** Add a `_transitions` map to the entity GET response
(alongside the existing `_actions` / `_fields` / `_relations` computed in
`internal/dataentry/entityserializer.go`), populated from
`PolicyResolver.TransitionVerdicts(ctx, e)`. Shape per machine field: `[{to,
guard, allowed, reason}]`. Read-only hint, like the other affordance maps — the
server re-enforces on the actual write (attempt-and-recover stays the backstop).
2. **SPA status control.** Render a state-machine field's transitions (not the
raw enum): each performable target as an enabled action; non-performable ones
disabled with the reason (guard / precondition) surfaced. Commit-on- select (a
transition is its own atomic PATCH, distinct from content edits). Generic over
ANY machine-typed field (keyed off the metamodel type having transitions), not
hand-authored per entity type. Falls back to the current enum `<select>` when
`_transitions` is absent (non-machine field / older server).
3. **Entry-locked create field** (closes the BUG-X1C7S UX gap). On the create
form, a state-machine field must NOT be freely editable — create is pinned to
the initial state (BUG-X1C7S). Render it read-only / showing the initial value /
omitted. Needs a create-side affordance hint (the machine's entry value) so the
form knows to lock it, mirroring how field affordances gate edit controls.

**Out:**
- CLI "what can I do from here" and mermaid export (separate consumers of the
same `Performable` accessor).
- A `transition:*` ACL wire verb / new `_actions` key — `_transitions` carries
the data directly; the verb-taxonomy question (TKT-XZEY) is not reopened here.
- Any change to enforcement or to the `statemachine`/`affordances` read query
(both landed in TKT-FT8J9; this ticket only consumes them).

## Design notes (to firm up in planning)

- **`_transitions` shape + serializer wiring** — mirror the `computeFields` /
field-verdict path; `TransitionVerdicts` returns `map[field][]TransitionVerdict`
which maps almost directly. Confirm the DTO/JSON tags and that it rides the same
ctx-scoped resolver call (no extra ACL round-trip).
- **SPA: which component** — the status control likely lives in
`FieldRenderer.vue` (machine-typed field → transition control instead of a plain
select) and/or a dedicated `StatusControl.vue`; the read view could also surface
it (`SidePanel.vue` already renders enum badges — could become the
commit-on-select control). Decide placement in planning; keep it generic.
- **No client-side ACL** (dataentry/CLAUDE.md): the SPA reads the server-computed
`allowed`/`reason` booleans; it does not evaluate guards or predicates. A
`reason: precondition` option is disabled with an explanatory tooltip, not
re-derived.
- **Entry-locked create** — the create response (or the form schema) needs the
entry value. `statemachine.Set` already has the compiled entry; expose it as a
read accessor (small, e.g. `EntryValue(type, prop)`) if not already, and thread
it into the create-form affordance.
- **Attempt-and-recover stays** — the control filters/gates optimistically; the
server 422/403 remains the source of truth. Never treat `_transitions` as
authorization.

## Acceptance criteria (firm up in planning)

1. Entity GET response carries `_transitions: map[field][]{to, guard, allowed,
reason}` computed from `TransitionVerdicts`, present only for machine-typed
fields, absent otherwise.
2. The SPA renders a machine-typed field as a transition control showing only
the declared out-edges: `allowed` ones selectable, others disabled with the
`reason`. Generic over any machine field; falls back to `<select>` when
`_transitions` absent.
3. Selecting a target commits the transition (atomic PATCH of the field);
a server 403/422 surfaces the existing structured error.
4. On the create form, a state-machine field is not freely editable — it is
locked to / shows the initial value; a user cannot submit a non-initial value
(server already rejects it — BUG-X1C7S — but the UI must not present it as
editable).
5. Wire + SPA tests: serializer emits `_transitions`; component renders
allowed/disabled correctly from a fixture; create form locks the field.

## References

- Consumes TKT-FT8J9 (`Set.Performable` / `PolicyResolver.TransitionVerdicts`) —
the dormant read query this makes visible.
- Create-lock rationale: BUG-X1C7S (create pinned to initial state).
- Affordance wire pattern + no-client-ACL rule: `internal/dataentry/CLAUDE.md`.
- Serializer: `internal/dataentry/entityserializer.go` (`_actions`/`_fields`/`_relations`).
- SPA affordance gating idiom: `FieldRenderer.vue` / `DynamicForm.vue`.
