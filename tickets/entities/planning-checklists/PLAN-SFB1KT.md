---
id: PLAN-SFB1KT
type: planning-checklist
title: 'Planning: Relation field-level ACL redaction (visible:) — currently absent for relations, live and history'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: relation-meta field-level `visible:` redaction on every
browser-reachable relation read (the `/relations` map, single-relation-type GET,
outgoing + incoming) and relation history (fail-closed, `history:read-redacted`
reveal). OUT: redacting the machine-to-machine sync channel (`/api/sync/`) — it
is a full-fidelity replication read-that-feeds-a-write; redacting it would erase
hidden fields on push. Documented as a deferred gap.

**Acceptance Criteria:**
1. A relation `visible:` grant hides non-granted meta keys on the live GET (all 4 shapes), keeps granted ones — `TestRelationRedaction_LiveGet_SelectiveStrip`, `_LiveIncoming_UsesSourceGrant`, `_NoBlock_Permissive`.
2. Relation history fails closed for subject-conditional grants — `TestRelationHistoryRedaction_SubjectConditional_FailsClosed`.
3. `history:read-redacted` holder sees frozen meta — `TestRelationHistoryRedaction_RevealPermission_ShowsFrozenMeta`.
4. Free-form (undeclared) meta keys are redacted under closed-world — `TestRelationVisible_FreeFormKey_ClosedWorld`.

## Research

- [x] ~~run `/research`~~ (N/A: bounded follow-up, design pre-agreed with 73C6B2)
- [x] Searched for existing patterns — mirrors entity `visible:` (RoleDef.Visible → FieldVerdicts → stripHiddenProperties)
- [x] Checked codebase for reusable code — reused `dimension` closed-world, `bindingFor`, `WithHistoricalSubject`, `relationSourceEntity`
- [x] Looked for reference implementations — the just-merged TKT-73C6B2 entity historical path is the template
- [x] Reviewed relevant concepts — audit-log, ACL visibility (DEC-ZBI39P)

**Research Doc:** N/A — coupled follow-up to TKT-73C6B2, shared design.

**Existing Solutions:** Entity side: `PolicyResolver.FieldVerdicts` Visible
dimension, `stripHiddenProperties` (affordances.go), `history_handler.go` reveal
branch. Relations reused all of this rather than a bespoke path.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**
1. Add `Visible []FieldGrant` to `RelationGrant` (parallel to write-side `Fields`).
2. `PolicyResolver.RelationFieldVerdicts(ctx, from, relType, metaKeys)` — resolves relation `visible:` through the same bindings; inherits historical neutering + a `relationTypesWithVisible` type-level closed-world.
3. `RelationVisibilityResolver` optional interface (type-asserted like `TransitionResolver`) so Nop/Demo need no method.
4. `visibleRelationMeta` / `visibleRelationMetaIncoming` (fail-closed) helpers; strip AFTER sort (sort reads the order key). Shared `buildRelationTypeRows` + `App.redactRelationMetaStrip` chokepoint.
5. Relation history wires the 73C6B2 seam (`WithHistoricalSubject` unless `history:read-redacted`).

**Alternatives rejected:** populating `entity.Relation.Inaccessible` (overloads
a git-crypt storage field with an ACL meaning — category error); a
relation-specific redaction path (ticket forbids it).

**Files to modify:** internal/acl/policy.go,
internal/affordances/{resolver,affordances}.go,
internal/dataentry/{affordances,affordances_policy,api_v1,relation_history_handler}.go,
docs.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** relation meta keys (from store), relation type +
fromType (URL segments). fromType for deleted-endpoint history is
caller-supplied and unvalidated — safe because the type-level closed-world keys
on `relType`, not `from.Type`, so a spoofed fromType still fails closed.

**Security-Sensitive Operations:** this IS the ACL redaction path. Fail-closed
everywhere: unresolvable historical subject-world → hidden; unresolvable
incoming source → hidden; empty role set on a `visible:`-gated type → hidden.
Restore reads RAW meta (never redact a read that feeds a write).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** 8 resolver tests (relation_visibility_test.go) + 8 handler
tests (relation_redaction_test.go) covering
live/incoming/history/reveal/free-form/fail-closed.

**Edge Cases:** free-form undeclared meta key; empty role set (type-level
closed-world); missing referenced property (binds Nil → hidden); incoming source
deleted mid-request (fail closed); deleted-endpoint + spoofed fromType.

**Negative Tests:** subject-conditional grant in history → hidden; non-reveal
principal → redacted; source gone → empty meta.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (m)

**Risks:** (1) new serialization site forgetting to redact — mitigated by the
shared `redactRelationMetaStrip` chokepoint. (2) sync-channel raw emit —
documented deferred gap (round-trip-safe redaction is the follow-up). (3) map
aliasing of store-owned edge.Properties — mitigated by copy-on-redact.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/acl-security.md + docs-project mirror — relation `visible:` redaction + sync deferred-gap. (Other docs N/A.)

## Design Review

- [x] Design pre-agreed as part of the coupled TKT-73C6B2 design discussion (deny-by-default, generic machinery relations plug into)
- [x] All critical/significant findings addressed — see code-review responses RR-AO1RFG, RR-JZ7VDI, RR-KTBK9G, RR-XCAI6J

**Design Review Findings:** covered via code-review (below); no separate
design-review round needed for this pre-scoped follow-up.
