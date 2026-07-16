---
id: PLAN-5BYO6
type: planning-checklist
title: 'Planning: Declarative status/enum state machines: transitions on CustomType with ACL-permission guards + predicate preconditions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (in: transitions block on CustomType, compile+enforce, guard/when; out: snapshots/freeze/approvals/publication — see ticket)
- [x] Acceptance criteria documented with specific test scenarios (7 ACs in ticket, each mapped to a test in IMPL-CP7L2 evidence)

Full design captured in `.ignored/designs/status-transitions.md`.

## Research

- [x] Searched for existing primitives (reused metamodel.CustomType, internal/predicate, acl RoleDef.Permissions/computeForEntity, entitymanager pipeline)
- [x] Checked codebase for similar patterns (automation engine's from/becomes old→new pairing; affordances RelationLookup/BindingContext for graph host funcs; delegate-permission gate as guard template)
- [x] Looked for reference implementations (OpenVWR SnapshotState machine; mermaid stateDiagram shape)
- [x] Reviewed rela concepts (authorization concept, TKT-XZEY parameterised-verbs reframe, RES-6PK0S3 evaluator convergence)

## Approach

- [x] Technical approach chosen and documented (transitions as data on CustomType; Compile→injected enforcer; required Deps collaborator run in the fixed write pipeline)
- [x] Approach builds on existing patterns (consumer-side interfaces; capability split; fail-fast constructor; predicate reuse)
- [x] Alternatives considered and rejected: guard-as-when-condition (hardcodes policy in schema); enforcement option A re-order whole path / option B automation-tier (both rejected in favor of option C dedicated step per RR-FPZJX); legality-in-ValidateEntity (rejected — ACL-in-metamodel layering); string `-->` DSL (rejected — parser to own)
- [x] Dependencies identified (entity, metamodel, predicate for statemachine; acl for the guard adapter; store for graph lookup)

## Security Considerations

- [x] Input sources identified (metamodel transitions at boot; property values + principal at write time)
- [x] Input validation approach defined (Compile fail-fast on dangling from/to/initial/default, dup edges, bad when: syntax, list-property)
- [x] Security-sensitive operations identified (the guard is authorization; subject-aware resolution required; delegate-gate self-promotion RR-7O6Q documented; guard fails CLOSED under an active policy when Request absent — RR-UOBUC)
- [x] Error handling doesn't leak sensitive info (denials carry permission name + reason, no policy topology)

## Test Plan

- [x] Test scenarios documented for each acceptance criterion (see IMPL evidence)
- [x] Edge cases identified (illegal skip/backward, guard denied, precondition false/missing, illegal entry, clear-to-empty, whitespace, list-property, Default typo, entity.value binding, subject-scoped role)
- [x] Negative test cases defined (Compile error table; guard-denied 403; illegal 422; no-orphan-on-rejection)
- [x] Integration test approach defined (entitymanager write path — Create/Update/ApplyEntity — with real Compile output)

## Risk Assessment

- [x] Technical risks assessed (unforgettability across ALL write paths — the ApplyEntity gap RR-NB135 was the key risk, closed; create-orphan ordering RR-HETEE closed)
- [x] Security risks assessed (fail-open guard RR-UOBUC → fail-closed; subject-scoping correctness RR-UJPW4)
- [x] Effort estimated (m)

## Documentation Planning

- [x] User-facing docs identified

**Documentation Impact:**

- [x] ~~Metamodel reference (docs/metamodel.md) for the `transitions:`/`initial:` block~~ (deferred: create a doc-task when the feature is user-announced; the godoc on `CustomType.Transitions`/`TransitionDef` documents it for now)
- [x] CLI help text — N/A (no new CLI command; `rela validate` lint surface is a nice-to-have, not built)
- [x] CLAUDE.md — N/A (no new cross-cutting rule; the design doc is the reference)
- [x] README.md — N/A
- [x] API docs — N/A (data-entry has no published OpenAPI spec; the 422/403 shapes reuse existing envelopes)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (all addressed):**

- RR-FPZJX (critical) — enforcement point lacks (from,to); dedicated post-old-load step. Addressed.
- RR-UJPW4 (significant) — subject-aware guard via computeForEntity, not holdsPermission. Addressed.
- RR-EU8GJ (significant) — graph-backed when: bindings (statemachine.GraphLookup adapter). Addressed.
- RR-1SMG4 (significant) — create-path entry semantics (default initial; non-initial → 422). Addressed.
- RR-NGBMT (minor) — named-type-only documented; list-property rejected. Addressed.
- RR-SGQO3 (minor) — build-vs-reuse rationale for edge-local when: documented. Addressed.
