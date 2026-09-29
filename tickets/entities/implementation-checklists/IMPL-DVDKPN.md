---
id: IMPL-DVDKPN
type: implementation-checklist
title: 'Implementation: Rename authorizes the zero face but re-keys every face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (internal/entitymanager/familyrename_acl_test.go)
- [x] Integration tests written (test full flow, not just units) (tests drive Manager.RenameEntity with the real Declarative ACL over memstore, fsstore, sqlite and postgres)
- [x] Happy path implemented (editor with update on both faces renames both, one rename version and audit record per face)
- [x] Edge cases from planning handled (face added before the Tx and inside it; dry run; rename onto a faced id; fs/memstore cannot roll back so the logs record what moved)
- [x] Error handling in place (errors surfaced, not swallowed) (family read errors fail closed before any write)

## Test Quality

- [x] Using fixture builders or factories for test data (newFacedPolicyFixture, policyFace)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (CLI `rela rename id` on a project with POL-1@draft and POL-1@published)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (CLI dry run on the faced entity now plans instead of reporting not-found)

**Verification Evidence:** `rela rename id POL-1 POL-2 --dry-run` printed the
plan; the real rename moved both files and wrote two rename-entity records,
"renamed face draft" and "renamed face published". Denial paths are verified by
TestFamilyRename_DeniedUnlessEveryFaceIsRenamable on all four backends.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors the BUG-1YN750 family delete)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (authorizeFamilyDelete generalized to
authorizeFamily(op) and shared by delete and rename)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
