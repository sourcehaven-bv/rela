---
id: PLAN-QQLK3F
type: planning-checklist
title: 'Planning: Machine-aware status control: surface _transitions on the wire + SPA performable-transition UI + entry-locked create field'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
1. `metamodel.TransitionDef.Label` (new optional YAML `label`) — display text for the *move* (transitions are actions, not states).
2. `statemachine`: carry label on compiled edge + `TransitionVerdict.Label`; add `Set.EntryValue(type, prop)` read accessor.
3. Wire: `v1.Entity._transitions` (`map[field][]Transition{to,label,guard,allowed,reason}`), pointer/closed-world like `_fields`.
4. dataentry: optional `TransitionResolver` interface, type-asserted in `attachEntityAffordances`; per-entity responses only. Create-lock via dry-run `_fields[field].writable=false` + entry value.
5. New SPA `StatusControl.vue`: shows only ALLOWED transitions, labeled by action label; commit-on-select via useAutoSave. FieldRenderer routes machine field → StatusControl, else SelectWidget (fallback).
6. Create form: machine field locked to initial value.

OUT: CLI/mermaid consumers; `transition:*` ACL verb; any enforcement / FT8J9
read-query change.

**Acceptance Criteria:**
1. Entity GET carries `_transitions` for machine fields (from TransitionVerdicts), absent otherwise + absent under Nop resolver. Test: serializer test with a machine-typed fixture vs non-machine.
2. SPA renders machine field as StatusControl showing ONLY performable moves, each action-labeled; non-machine falls back to SelectWidget. Test: component test from `_transitions` fixture.
3. Selecting a move commits an atomic field PATCH; 403/422 surfaces existing structured error. Test: StatusControl commit test asserts scheduleFieldSave called.
4. Create form: machine field not freely editable (locked to initial). Test: create-form/dry-run lock test.
5. `TransitionVerdict.Label` fallback: transition label → state label → raw value. Test: statemachine resolve test.

## Research

- [x] ~~run `/research`~~ (N/A: pure consumer of merged TKT-FT8J9 + small additive schema field)
- [x] Searched codebase for similar patterns
- [x] Reviewed prior art

**Existing Solutions:**
- `SelectWidget.vue` already consumes a `transitions: Record<current, next[]>` prop and gates options — but fed from STATIC form config, not the wire. Reuse the seam concept, but "only-allowed + action labels" is a genuinely different control → new `StatusControl.vue`.
- Optional-capability type-assert pattern (store's `HistoryReader`/`Formatter`) → `TransitionResolver` sibling interface, same idiom.
- `useAutoSave.scheduleFieldSave` = existing atomic single-field PATCH path (commit-on-select).
- `statemachine.Set` already holds compiled `m.entry` → expose as `EntryValue`.

## Approach

Backend: TransitionDef.Label → edge.label → TransitionVerdict.Label
(display-only, no enforcement change). `_transitions` on v1.Entity, filled in
attachEntityAffordances via a type-asserted `TransitionResolver`. Create-lock
threads entry via dry-run `_fields`.

SPA: `_transitions` on Entity type; new StatusControl.vue (menu of
only-performable, action-labeled, commit-on-select); FieldRenderer routes to it
when `_transitions[field]` present.

**Files to modify:**
- `internal/metamodel/types.go` (TransitionDef.Label)
- `internal/statemachine/{statemachine,compile,resolve}.go` (edge.label, Verdict.Label, EntryValue)
- `internal/affordances/resolver.go` (Label passthrough — already returns statemachine.TransitionVerdict, so free)
- `internal/apiwire/v1/responses.go` (Entity.Transitions, Transition)
- `internal/dataentry/{affordances,affordances_policy,affordances_stub,entityserializer}.go` (TransitionResolver, attach)
- `frontend/src/types/entity.ts`, `frontend/src/components/forms/StatusControl.vue`, `FieldRenderer.vue`, `DynamicForm.vue`
- `docs/data-entry/api-reference.md`

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `_transitions` is server-COMPUTED (from
TransitionVerdicts under the request principal) — not client input. `label` is
operator-authored metamodel config (trusted, like existing `Labels`). The SPA
reads server booleans; NO client-side ACL (dataentry/CLAUDE.md).
Attempt-and-recover: server re-enforces every write — `_transitions` is a UI
hint, never authorization.

**Security-Sensitive Operations:** The transition write is the existing
entitymanager write path (already guard/precondition-enforced by TKT-E4LW2).
This ticket adds NO new write path — StatusControl PATCHes
`{properties:{field:to}}` through the same gated update. A tampered
`_transitions` can only widen the *displayed* options; the write still 403/422s.

## Test Plan

- [x] Test scenarios documented per AC (above)
- [x] Edge cases identified
- [x] Negative cases defined
- [x] Integration approach defined

**Test Scenarios:** see AC1–5.

**Edge Cases:**
- Terminal state (no out-edges) → `_transitions[field]` empty/absent → StatusControl shows current only, no moves.
- No label on TransitionDef → falls back to state label then raw `To`.
- Non-machine enum field → no `_transitions` key → SelectWidget fallback (unchanged).
- Nop/Demo resolver (no TransitionResolver) → `_transitions` absent entirely.
- All out-edges non-performable (guard/precondition) → menu empty (current only) — the "only allowed" contract.

**Negative Tests:** picking a move the server rejects (race: verdict stale) →
403/422 structured error toast; UI does not treat `_transitions` as
authoritative.

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- Label field touches metamodel+statemachine (beyond "pure consumer") — mitigated: additive, optional, fallback chain; no enforcement change; user approved inclusion.
- New component vs reuse — mitigated: StatusControl is small, FieldRenderer falls back to SelectWidget so no regression for non-machine fields.

Effort: l (confirmed).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist created on entering implementation

**Documentation Impact:**
- [x] docs/data-entry/api-reference.md — `_transitions` wire shape
- [x] docs/metamodel.md — TransitionDef `label`
- [x] ~~other docs~~ (N/A: no README/tutorial/CLI impact)

## Design Review

- [x] ~~Run `/design-review`~~ (design settled via direct user decisions on control shape + label; approach is mechanical consumption of merged primitives)
- [x] No open critical/significant findings

**Design Review Findings:** N/A — plan approved directly by user (only-allowed
shape, action labels, new StatusControl, include TransitionDef.label).
