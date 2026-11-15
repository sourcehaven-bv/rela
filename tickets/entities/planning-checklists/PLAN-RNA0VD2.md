---
id: PLAN-RNA0VD2
type: planning-checklist
title: 'Planning: Relation checks in enforce: write validation rules'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `related(entity, 'rel')` and `related(entity, 'rel', {type =
'x'})` in the `when`/`then` expressions of an `enforce: write` rule, one hop, no
property or `id` constraint, on a type without `faces:`. Checked on entity
update and patch, relation create, relation delete (plain and If-Match) and
`ReplaceRelations`. Out: creates (entity, batch, linked, copy, conversion,
restore), entity deletes, automation and conversion edge writes, `relations:`,
`count_relations`/`has_relation`, multi-hop paths, constraints on the far
entity.

**Acceptance Criteria:**
1. An update that makes a satisfied relation rule fail is refused with 422 and leaves the row unchanged (entitymanager relation-rule update test).
2. An entity that already breaks the rule stays editable (legacy-row case).
3. Deleting the last edge the rule needs is refused; deleting one of two is allowed (relation delete test, plain and If-Match, and ReplaceRelations).
4. Create an entity in the rule's state, then link it: both writes succeed (create-then-link test).
5. An inverse relation name counts incoming edges (inverse case).
6. A hidden neighbour counts; the refusal names the rule only (ACL test).
7. Multi-hop, property constraints and faced types fail at load (writerule TestNewSet_Refuses).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: extends the existing rule shape; the design is in the ticket body)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal rule engine)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal feature)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `statemachine` evaluates `related(...)` in transition
preconditions on the write path through a raw-store binder.
`predicatefns.ResolveTraversal` is the one hop resolution.
`checkRelationCapacity` counts edges inside the relation write's Tx and decides
before writing, which is the model for a check that must not depend on rollback.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `writerule` compiles a rule with `related(...)` as a
relation rule: it resolves each hop and keeps it out of `Check`. A new
`CheckChange(ctx, before, after, was, now)` evaluates the rule with the
traversal answered by a consumer-side `Edges` interface and returns a violation
only when `before` with `was` satisfied the rule and `after` with `now` does
not. `entitymanager` implements `Edges` over the Tx view: one `ListRelations`
per hop and one `MatchingIDs` per far type, with the pending edge changes
applied for `now`. All checks run in the Tx before any write, so a refusal
writes nothing on every backend, including fs and mem, which do not roll back.

Alternatives rejected: checking every write that leaves the rule broken (an
entity created in the rule's state could never be linked edge by edge, and
legacy rows would lock); write-then-check (fs and mem cannot roll back);
evaluating after the whole request (no request scope in the manager; the SPA
writes edges as separate requests); counting only visible edges (the verdict
would depend on the writer and disagree with `rela analyze validations`).

**Files to modify:** internal/writerule/writerule.go,
internal/entitymanager/{relationrules.go,manager.go,cardinality.go,ifmatch_delete.go},
internal/dataentry/write_handler.go, internal/metamodel/{types,loader}.go
(comments), internal/store/storetest/tx.go,
docs-project/entities/guides/GUIDE-metamodel.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Rules come from operator-authored schema.yaml.
The allowed `related(...)` shape is an allowlist checked at load. Edges come
from the store.

**Security-Sensitive Operations:** The check reads raw edges, as the write path
does. The 422 names the rule and its description, never an entity id. The one
bit left (whether an entity the caller may write has an edge of the type, hidden
or not) is documented. Entity deletes are not checked, so a delete never reveals
a hidden neighbour's rule.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Unit tests in writerule (CheckChange table, refusals);
integration tests through a real Manager on memstore and fsstore; a storetest
case for graph queries reading a Tx's own writes on every backend.

**Edge Cases:** Union target with `type =`; dangling edge to a missing entity;
self-referencing relation; both ends of one edge carry a rule; rule with no
`entity_type`.

**Negative Tests:** Multi-hop, property and id constraints, faced type,
`relations:` still refused; violated rules refuse writes and leave the store
unchanged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Relation writes on relation-rule types now open a Tx (write lock):
bounded to relation types a rule reads. Writes that are not checked (entity
delete, batch edges, automations) can leave a rule broken; analyze reports it
and the next update of the entity does not lock. Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md - "Enforcing a rule on write (enforce: write)"

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the written design in the ticket body covers the decisions the request named; the code review covers the rest)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
