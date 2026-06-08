---
id: REV-IZGK
type: review-checklist
title: 'Review: acl: Subject + Source + Request + resolver (declarative role-based authz)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full tree green; race-clean for
acl/entitymanager/dataentry packages.
- [x] Lint clean (`just lint`) — golangci-lint reports 0 issues.
- [x] Coverage maintained (`just coverage-check`) — package floor and
total (74.3%) PASS.
- [x] arch-lint clean (`just arch-lint`) — extended `acl.mayDependOn`
with `entity` + `store` (required by storegraph.go + readquery.go).

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~
(N/A: per `.ignored/acl-v1-split-plan.md`, the reference branch
`feat/acl-v1-tkt-svxl` already underwent two cranky-code-reviewer passes
producing 22 review-response findings. This PR ports the already-reviewed code;
rather than re-run review, this checklist audits which of those findings apply
to PR-2 scope and confirms they're addressed in the ported code.)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses (audit of source-branch findings applicable to PR-2 scope):**

The split plan identifies these reference-branch findings as in-scope for PR-2.
Each has been verified addressed in the ported code:

| Finding ref | Issue | Verified in PR-2 code |
|-------------|-------|----------------------|
| RR-X1TE (significant) | Delete Subject==nil legacy fallback | `acl/authz_write.go` panics on nil Subject; `acl/acl.go` doc explains the contract. `TestAuthorizeWrite_NilSubject_Panics` enforces. |
| RR-NIGK (significant) | `Policy.Validate()` rejects blanks | `acl/policy.go` `Validate()` returns error for blank role/type/relation. `TestPolicy_Validate_RejectsBlanks` covers. |
| RR-MBK0 (significant) | Sort `RoleRelations` iteration | `acl/resolver.go` walks sorted keys. `TestResolver_RolesDeterministic` (50×) pins ordering. |
| RR-L3VO (significant) | `HasEdge` uses `GetRelation` | `acl/storegraph.go` calls `store.GetRelation` (not list+filter). Surfaces non-NotFound errors. |
| RR-9GN3 (significant) | `Policy()` immutability doc | `acl/declarative.go` godoc on `Policy()` declares the return is for read-only inspection. |
| RR-JJYW (significant, WithRequest part) | `acl.WithRequest`/`FromContext` exist | `acl/request.go` exports both helpers. Middleware wiring deferred to PR 4. |
| RR-AROE (minor, acl side) | `DepthCap` exported in acl | `acl/internals.go` exports `DepthCap`. `TestDepthCap_LockstepWithGraphquerynaive` asserts equality with `graphquerynaive.DepthCap`. |
| RR-3D6Q (minor) | `World.Visible` nil-Query check | `acl/readquery.go` guards nil-Query path. |
| RR-2XZW (minor) | AssertHidden/AssertContains existence pre-check | `acl/testutil_test.go` helpers check entity exists before asserting visibility. |
| RR-K3OO (minor) | HasEdge surfaces non-NotFound errors | `acl/storegraph.go` wraps non-NotFound store errors. `TestStoreGraph_HasEdge_SurfacesUnexpectedError`. |
| RR-F9M9 (nit) | Drop `RelationSubject.To*` if unused | `acl/subject.go` `RelationSubject` only carries Type/FromID/ToID needed for actual checks; no spurious fields. |
| RR-ZB1V (nit) | role_relations whitespace test | `acl/policy_test.go` covers whitespace-only names. |
| RR-79HD (nit, entitymanager piece) | `recordDeniedWrite` Subject.ID/FromID | `entitymanager/manager.go` records `Subject.ID` for entities, `Subject.FromID` for relations in the audit `denied-write` row. `entitymanager/acl_test.go` `TestEntityManager_DeniedWrite_RecordsSubjectAttribution` verifies. |

**Deferred to follow-up PRs (out of PR-2 scope):**

- RR-72OJ (critical) — appbuild fail-loud on malformed acl.yaml → PR 4
- RR-WTLD (significant) — affordances.New drops policy arg → PR 3
- RR-JRPZ (significant) — has_role consults entityRoles → PR 3
- RR-FGJR (significant) — `Collaborators.Declarative` + appbuildtest → PR 4
- RR-36UL (significant) — `WithACL` auto-detect → PR 4
- RR-Y6Y9 (minor) — AC8 parity test rewritten discriminating → PR 3
- RR-8ZGO (minor) — middleware respects upstream Request → PR 4
- RR-K7CT (minor) — TestResolver_ReusesRequestFromContext premise → PR 3
- RR-7O6Q (nit) — member-of doc → PR 4 (docs/security.md)
- RR-JJYW (middleware part) → PR 4

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All 11 acceptance criteria from PLAN-OJSS verified PASS — see the "AC mapping"
table in IMPL-DQ2V for the test name and result for each.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A:
internal refactor, no user-facing docs change in this slice; user- facing docs
land with PR 4 wiring)
- [x] ~~User-facing documentation updated~~ (N/A: see above)
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

**Docs Checklist:** N/A — internal refactor (kind=refactor on the ticket).

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/905 (base:
`feat/store-graphquery-dsl`; stacks on PR 1 /
[#903](https://github.com/sourcehaven-bv/rela/pull/903))
