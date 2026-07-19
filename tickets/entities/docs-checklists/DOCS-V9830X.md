---
id: DOCS-V9830X
type: docs-checklist
title: 'Docs: Machine-aware status control (TKT-3G93B8)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc / TSDoc on new exported symbols (statemachine.EntryValue, TransitionVerdict.Label, v1.Transition, TransitionResolver, computeTransitions, applyCreateLock, StatusControl.vue, adoptLockedFieldValues)
- [x] Non-obvious decisions explained inline (visibility gating rationale, create-is-entry-not-transition, stale-menu reload)

## Project Documentation

- [x] docs/data-entry/api-reference.md — new `## Transition affordances: _transitions` section (wire shape table, create-lock semantics, UI-hint-not-authorization note)
- [x] docs/metamodel.md — new `### State Machines (transitions)` subsection under Custom Types (transitions/initial/guard/when/label, with the new `label` field documented)

## External Documentation

- [x] ~~README / tutorials~~ (N/A: incremental affordance surface, covered by api-reference + metamodel guide)

## Verification

- [x] Docs cross-reference each other (metamodel `label` → api-reference `_transitions`; api-reference → metamodel state-machine section)
- [x] Wire-shape example matches the actual v1.Transition JSON tags
