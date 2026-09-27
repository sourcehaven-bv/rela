---
id: IMPL-WCM1R3
type: implementation-checklist
title: 'Implementation: related(): traverse incoming edges (filter B on properties of A where A -> B)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (predicate Key/Subject, predicatefns
  ResolveTraversal direction and refusals, acl direction placement and
  refusals, queryplan inverse and scope-derived indexes, appbuild Filter and
  traversalFunc, conditionlint refusals, boot field refusal)
- [x] Integration tests written (test full flow, not just units):
  storetest inbound endpoint-match cases on mem, fs, sqlite and pg;
  dataentry HTTP list tests under ACL and without; count-zero hint; query
  budget; pg EXPLAIN on the MatchingIDs shape
- [x] Happy path implemented
- [x] Edge cases from planning handled (symmetric, empty From, other
  subject, non-string and empty literals, named faces, hidden far rows,
  negation, inherited reads, slot collision)
- [x] Error handling in place (errors surfaced, not swallowed): only DenyAll
  reads as no match; every other refusal is an error

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Project in /tmp/cxqev0/proj: features FEAT-001..003, each implemented by one
ticket (in-progress, done, in-progress). Scopes `busy` and `idle` on feature
use `related(entity, 'implementedBy', { status = 'in-progress' })`.

- No acl.yaml, rela-server list API: `busy` = FEAT-001, FEAT-003; `idle` =
  FEAT-002.
- With acl.yaml (alice reads all; USR-001 reads features plus TKT-001 via
  editor-of; carol reads features only), requests interleaved:
  alice busy = 001, 003 and idle = 002; USR-001 busy = 001 and idle = 002,
  003; carol busy = none and idle = all three; alice again unchanged.
- A `visible:` list hiding ticket.status: rela-server refuses to start,
  naming the scopes, property and type.
- `related()` in a list `condition:`: `rela validate` fails with
  `lists["cond"]: related(...) is only supported in query_scopes`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities: one resolver (ResolveTraversal) for
  validation, index derivation and lowering; one lowering (lowerTraversal)
  for gated and ungated; one StringShaped definition in metamodel
- [x] No security issues introduced: hidden far rows do not count; hidden
  fields refused at boot and per request; default state only
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
