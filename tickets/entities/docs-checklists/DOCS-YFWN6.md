---
id: DOCS-YFWN6
type: docs-checklist
title: 'Documentation: Resolved transition affordance'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code documentation

- [x] Public Go types/methods have doc comments (`statemachine.TransitionVerdict`, `VerdictGate` + constants, `Set.Performable`, `Set.MachineProps`; `affordances.PolicyResolver.WithMachines`, `TransitionVerdicts`)
- [x] Non-obvious WHY comments (why Performable evaluates against the post-transition clone — read/write parity; why self-loops are skipped; why the read guard fails closed with no inert tier; why predicate-on-read is bounded/acceptable; the double-Compile determinism+immutability rationale)
- [x] The extracted `evalEdge` documents that it is the single source of truth shared by write `applyEdge` and read `Performable` (the drift guard)

## Project documentation

- [x] CLAUDE.md — updated (root + `internal/entitymanager/`) to clarify the no-predicate-on-reads boundary (hot/unbounded list vs bounded single-field), which is what makes this read query legitimate.
- [x] README.md — N/A (internal API)

## External documentation

- [x] ~~Metamodel / API reference~~ (N/A: the query is dormant — no wire surface yet. When the SPA status control / `_transitions` wire key lands in the consumer ticket, a doc-task documents the wire contract then.)
- [x] API docs — N/A (no wire surface in this ticket)

## Notes

- Ships **dormant**: delivers `Performable`/`MachineProps` + `TransitionVerdicts` (the data), no production caller yet. Wire surface + SPA control are the linked follow-up. Fully tested at the unit/integration level including the AC4 read/write parity guard.
