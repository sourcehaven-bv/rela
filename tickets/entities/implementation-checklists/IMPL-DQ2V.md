---
id: IMPL-DQ2V
type: implementation-checklist
title: 'Implementation: acl: Subject + Source + Request + resolver (declarative role-based authz)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Code ported from `feat/acl-v1-tkt-svxl` with surgical adjustments for the PR-2
slice. Files added/modified:

- New: `internal/acl/{subject,source,source_test,graph,storegraph,
request,resolver,resolver_test,authz_write,internals,readquery,
features_test,doc_test,testutil_test}.go`
- Modified: `internal/acl/{acl,declarative,declarative_test,doc_test,
nop_test,policy,policy_test,readonly_test}.go`
- Modified: `internal/entitymanager/manager.go` (Subject population,
audit attribution surfacing)
- New: `internal/entitymanager/acl_test.go` (denied-write attribution)
- Adjusted for PR-2 compile: `internal/appbuild/appbuild.go`
(`loadACL` calls `acl.NewDeclarative(policy, acl.NullGraph{})`; full store-graph
wiring deferred to PR 4 per plan), `internal/dataentry/affordances.go`
(translateVerb takes entityID, populates `acl.EntitySubject`),
`internal/dataentry/affordances_test.go` (call-site updates),
`cmd/rela-server/main_acl_test.go` (call-site updates).
- Arch-lint: `.go-arch-lint.yml` extended `acl.mayDependOn` with
`entity` + `store` (needed by `storegraph.go` and `readquery.go`).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Test files ported from the reference branch (already vetted by the
cranky-code-reviewer pass that produced the 22 RR entities). New tests this PR
adds for acceptance:

- `TestAuthorizeWrite_NilSubject_Panics`
- `TestAuthorizeWrite_UnstampedPrincipal_Denies`
- `TestRequest_ForEntity_AttributionsDeterministic` (50 iterations)
- `TestDepthCap_LockstepWithGraphquerynaive`
- Plus all the existing acl tests pass against the new shape.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Automated verification (the acl/entitymanager surface is internal — no manual UI
to drive):

- `go test ./...` — full tree green (60+ packages, all `ok`).
- `go test -race ./internal/acl/... ./internal/entitymanager/... ./internal/dataentry/...` — race-clean.
- `go test -run "TestAuthorizeWrite_NilSubject|TestAuthorizeWrite_UnstampedPrincipal|TestRequest_ForEntity|TestDepthCap" -v ./internal/acl/...`:
  - `TestDepthCap_LockstepWithGraphquerynaive` PASS
  - `TestRequest_ForEntityReusesGlobals` PASS
  - `TestRequest_ForEntity_AttributionsDeterministic` PASS
  - `TestAuthorizeWrite_NilSubject_Panics` PASS
  - `TestAuthorizeWrite_UnstampedPrincipal_Denies` PASS

AC mapping (per planning checklist AC1–AC11):

| AC | Test name(s) | Status |
|----|--------------|--------|
| 1  | TestSubjectSum (in source_test.go) | PASS |
| 2  | TestPrimarySource_DeterministicTieBreak | PASS |
| 3  | TestRequest_ForEntityReusesGlobals | PASS |
| 4  | TestStoreGraph_HasEdge_SurfacesUnexpectedError | PASS |
| 5  | TestNewDeclarative_RequiresGraph (declarative_test.go) | PASS |
| 6  | TestPolicy_Validate_RejectsBlanks | PASS |
| 7  | TestResolver_RolesDeterministic | PASS |
| 8  | TestDepthCap_LockstepWithGraphquerynaive | PASS |
| 9  | TestAuthorizeWrite_NilSubject_Panics, TestAuthorizeWrite_UnstampedPrincipal_Denies | PASS |
| 10 | TestEntityManager_DeniedWrite_RecordsSubjectAttribution (in entitymanager/acl_test.go) | PASS |
| 11 | TestRequest_ForEntity_AttributionsDeterministic | PASS |

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Code-quality notes:

- `Subject == nil` panics in `AuthorizeWrite` (RR-X1TE) — no silent
allow/deny fallback that would mask programmer error.
- `StoreGraph.HasEdge` returns wrapped non-NotFound errors (RR-K3OO);
no silent "no role" on transient store failures.
- `loadACL` in appbuild **does** still tolerate-warn-on-parse-failure
(degrades to NopACL). RR-72OJ (fail boot on malformed acl.yaml) is intentionally
deferred to PR 4 where the proper `loadACLPolicy` + `buildACL` split lands.
- arch-lint passes; lint passes (0 issues).
