---
id: DOCS-STOGZ
type: docs-checklist
title: 'Documentation: Declarative status/enum state machines'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code documentation

- [x] Public Go types have doc comments (`CustomType.Transitions`/`Initial`, `TransitionDef`; `statemachine` package doc + `Set`, `Machine`, `Guard`, `GraphLookup`, `TransitionWiring`, `GuardError`, `Compile`, `EmptySet`, `EnforceUpdate`/`EnforceCreate`; `entitymanager.TransitionEnforcer` + new Deps fields; `acl.Request.HoldsPermissionForEntity`; `appbuild.CompileTransitions`)
- [x] Non-obvious WHY comments added (why enforcement runs post-old-load pre-write; why guard uses computeForEntity not holdsPermission; why fail-closed under active policy; why entity.value not hardcoded status; why EnforceCreate is inside createCore; why list-property rejected)
- [x] Design rationale captured in `.ignored/designs/status-transitions.md` (guard/when split, subject-scoping-via-topology, served-vs-direct tier, build-vs-reuse for edge-local when:)

## Project documentation

- [x] ~~CLAUDE.md rule~~ (N/A: no new cross-cutting rule; the design doc + godoc are the reference. Adding one would be aspirational per CLAUDE.md "no premature abstractions".)
- [x] README.md — N/A (project overview; transitions are an internal metamodel feature)

## External documentation

- [x] ~~Metamodel user reference (docs/metamodel.md) for `transitions:`/`initial:`~~ (N/A/deferred: the feature has no live consumer in-tree yet — it's the transition primitive extracted from the approval-lifecycle proposal, not yet wired into a shipped metamodel. When a project adopts it, a doc-task should add the `transitions:` block to docs/metamodel.md with the guard/when/initial semantics. The godoc documents it for developers today.)
- [x] API docs — N/A (data-entry has no published OpenAPI spec; the 422/403 responses reuse existing `writeV1Error` / `ForbiddenError` envelopes — the guard denial is a standard 403 with `rule_kind: transition-guard`)

## Notes

- The feature is additive and inert by default: a metamodel with no `transitions:` compiles to an empty enforcer and every write behaves exactly as before. No migration required for existing projects.
- Enforcement is served-path only for the guard (403); legality (422) applies wherever the entitymanager runs. Direct CLI/editor writes with no policy are ungated by design (git is the audit trail there).
