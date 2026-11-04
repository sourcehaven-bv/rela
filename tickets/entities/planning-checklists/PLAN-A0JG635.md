---
id: PLAN-A0JG635
type: planning-checklist
title: 'Planning: Reject type-disallowed relation edges on every write surface'
started: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- The data-entry REST write paths: `POST /api/v1/{plural}` and
  `PATCH /api/v1/{plural}/{id}` with a `relations` body, and the per-edge
  `POST .../relations/{rel}` endpoint.
- The manager's create path (`Manager.CreateRelation`,
  `Manager.ReplaceRelations`), which MCP, Lua and CLI already use.
- Removing `entity.RelationOptions.TolerateTypeMismatch`, the one opt-out.
- A new decision superseding DEC-HWZHA's type-mismatch classification;
  `internal/dataentry/CLAUDE.md`; `docs-project` API reference.
- Relations in this repo's own projects that violate their schema.

Out of scope:

- Other DEC-HWZHA soft conditions (missing target on validation,
  `target_type_mismatch` between the declared and the stored peer type,
  unknown/required/mistyped meta). They stay warnings.
- `UpdateRelation` (meta-only update of an existing edge): it creates no
  edge, and the owner states no broken edges exist.
- Entity conversion and copy: conversion already drops edges the new type
  does not admit; copy keeps the source's own type.
- A strict opt-in mode: the owner decided against it.

**Acceptance Criteria:**

1. PATCH whose relation body links a peer of a type the relation does not
   allow answers 422 with `target_type_not_allowed` (peer is the target) or
   `source_type_not_allowed` (peer is the source), stores no edge, does not
   update the entity's properties, and writes no audit record.
2. PATCH on a path entity whose own type is not in the relation's `from`
   (outgoing name) or `to` (inverse name) answers 422 with the matching code.
3. POST create with such a relation answers 422 and creates no entity.
4. Per-edge `POST .../relations/{rel}` answers 422 with the same codes.
5. Criteria 1-4 hold for canonical and inverse names, and for bounded
   (`max_outgoing`, `max_incoming`) and unbounded relations.
6. Manager `CreateRelation` and `ReplaceRelations` refuse a type mismatch
   with `*InvalidRelationError` carrying a stable code, and nothing is
   written; there is no option to tolerate it.
7. Missing target, unknown meta and required meta still answer 200 with
   warnings.
8. `tickets/`, `docs-project/` and the prototype projects contain no relation
   that violates their schema.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the owner fixed the approach; no options to weigh)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal validation rule)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal validation rule)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `metamodel.Validator.ValidateRelation` already decides the allowlist and
  returns `*metamodel.InvalidRelationError`.
- `prepareRelationFrom` (internal/entitymanager/manager.go) already refuses a
  mismatch unless `TolerateTypeMismatch` is set. Only dataentry sets it
  (relations_modern.go `writeCreateRelation`, `writeBoundedOps`). So the
  single rule already lives in the manager; MCP (`tools_relation.go`), Lua
  (`runtime.go`, `elevation.go`), CLI (`link.go`) and the per-edge REST POST
  already reject.
- `validateRelationsModern` is the pre-write validation phase for PATCH. It
  computes the same allowlist verdicts as warnings.
- DEC-HWZHA, TKT-9DINM3 (POST/PATCH disagreement; this ticket resolves it the
  other way), BUG-X1479P / GitHub #1806 (introduced `TolerateTypeMismatch`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. metamodel: give `InvalidRelationError` a `Code` field set by
   `ValidateRelation`: `source_type_not_allowed` or
   `target_type_not_allowed`, as exported constants.
2. entitymanager: delete the tolerate branch in `prepareRelationFrom`.
   `InvalidRelationError` exposes `Code()` (the metamodel code, or
   `unknown_relation_type`). `ReplaceRelations.prepare` keeps authorizing the
   other creates and removes before returning the allowlist error, so a
   denial is still reported as a denial; nothing is written either way.
3. entity: remove `RelationOptions.TolerateTypeMismatch` and its doc.
4. dataentry:
   - `validateRelationsModern` turns both allowlist verdicts (path side and
     peer side) into a `*structuralError` (422) with the code. The path-side
     check applies only when the wrapper upserts at least one edge; a body
     that only removes or clears creates nothing to refuse. The check runs
     before any write, and PATCH runs this phase before the entity update.
   - POST create runs Phase A after `CreateEntity`. Add a pre-check,
     `requireAllowedRelationTypes`, that runs the same allowlist checks on
     the candidate's type before `CreateEntity`. It shares one helper with
     `validateRelationsModern` so the two cannot drift.
   - `writeCreateRelation` / `writeBoundedOps` drop the tolerate flag; a
     manager `*InvalidRelationError` (a race where a peer was converted
     between check and write) maps to 422 with its code.
   - Per-edge `handleV1CreateRelation` maps `*InvalidRelationError` to 422
     with its code instead of the generic `relation_failed`.
5. Data: remove `BUG-L1DHC5 implements FEAT-870YCY` (the bug already has
   `fixes`) and `DEC-LFSYNY decides FEAT-SQLBK1` (no decision→feature
   relation exists; the decision already decides `store-backends`).

Alternatives rejected:

- Per-transport checks only: the manager already holds the rule; duplicating
  it per transport is what let REST drift.
- Strict opt-in header / query flag: rejected by the owner.

**Files to modify:**

- internal/metamodel/errors.go, validation.go
- internal/entity/writeapi.go
- internal/entitymanager/manager.go, cardinality.go (+ tests)
- internal/dataentry/relations_modern.go, write_handler.go (+ tests)
- internal/dataentry/CLAUDE.md
- docs-project entity backing docs/data-entry/api-reference.md
- tickets/: new decision, DEC-HWZHA update, two relation files removed

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The relation body of REST writes. Validation is the schema allowlist
(`from`/`to`). The peer type comes from `visibleReader.readableTypes`, so a
peer the caller may not read has no type and gets the existing
not-found answer; the 422 cannot reveal a hidden entity's type or existence.

**Security-Sensitive Operations:**

The 422 now precedes the manager's ACL check for PATCH/POST bodies. That is
safe: it depends only on the caller-known path type, the schema (not secret)
and readable peers' types. The audit log records no write for a refused
request.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1/2/5: table-driven `TestRelationWrite_RejectsDisallowedTypes` in
  dataentry over {canonical, inverse} × {bounded max_outgoing, bounded
  max_incoming, unbounded} × {wrong peer, wrong path entity}, asserting 422 +
  code, edges unchanged, entity properties unchanged, no write audit.
- AC3: POST create with a disallowed edge: 422, entity absent.
- AC4: per-edge POST with wrong target type: 422 `target_type_not_allowed`.
- AC6: flip `TestReplaceRelations_TolerateTypeMismatch` to assert rejection,
  no edge, no write audit; manager `CreateRelation` test for the code.
- AC7: existing warning tests (target_not_found, unknown_meta_key,
  required_meta_unset) keep passing.
- Flip `TestPatchRelations_RepointToDisallowedTypeWarns`,
  `TestPatchRelations_DisallowedTypeOnBoundedRelationAudited`, and adjust the
  api_v1/acl bypass tests that expected warnings.
- AC8: a script over each project's relation files and schema; `rela analyze`
  gates on tickets.

**Edge Cases:**

- A body with `data: []` or only `remove` on a relation the path type cannot
  source: no edge is created, so it is not refused.
- A peer the caller cannot read: keeps the not-found answer, no type leak.
- A full linkage mixing allowed and disallowed peers: whole request refused,
  nothing written.

**Negative Tests:**

All scenarios above are negative tests; the positive paths are covered by
the existing relation test suites.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Clients relying on the permissive behaviour start receiving 422. Accepted
  by the owner; documented in the decision and API reference.
- Existing broken edges in deployments: a PATCH whose `data` lists one is
  refused. Accepted by the owner (no carve-out); `rela link`/`unlink`
  can repair.
- e2e or fixture data relying on tolerance: checked statically; CI e2e
  confirms.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- docs-project source of docs/data-entry/api-reference.md (warning table and
  error codes).
- internal/dataentry/CLAUDE.md validation table.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-9OI141T (minor, peer-side code by canonical
side), RR-QN7IOTA (minor, 422 before 403 is safe; denied-remove test keeps an
allowed target), RR-PWYFN3V (nit, manager re-validates so a race maps to the
same 422). No critical or significant findings.
