---
id: IMPL-SGX0XX
type: implementation-checklist
title: 'Implementation: Access skeleton: mode decisions and request lifecycle'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (access/mode_test.go pins every mode and decision cell; access/opener_test.go covers attach, reuse, mismatch, unresolved, principal_property and Restamp; acl/verb_test.go; aclmap/verbs_test.go)
- [x] Integration tests written (test full flow, not just units) (existing dataentry router, provision, ceiling and asserted-identity e2e tests run through the Opener; TestGoldenParity passes without -update; cli/acl_verbs_test.go pins the CLI enums)
- [x] Happy path implemented (router and provision use access.Opener; five switches call access.Mode)
- [x] Edge cases from planning handled (typed-nil NopACL/ReadOnlyACL, nil *Declarative, unknown impl, empty permission, principal mismatch, rename excluded from reports)
- [x] Error handling in place (errors surfaced, not swallowed) (sentinels ErrPrincipalMismatch and ErrUnresolvedPrincipal wrap the cause; router maps them to the same 500 codes)

## Test Quality

- [x] Using fixture builders or factories for test data (acl.NewDeclarative with a lookup func; existing dataentry fixtures)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: pure refactor with no behaviour change; the golden wire corpus is the end-to-end check)
- [x] Each acceptance criterion verified with test scenario from planning (golden corpus byte-identical; archguard aclmode guard green; arch-lint green)
- [x] ~~Edge cases manually verified~~ (N/A: covered by the mode and opener table tests)

**Verification Evidence:** TestGoldenParity passes without -update. Race tests
pass for access, acl, aclmap, archguard, docs, dataentry and cmd/rela-server.
Non-test diff is about 890 lines, under the 1500-line cut, so no cut was needed.

## Quality

- [x] Code follows project patterns (check similar code) (constructor rejects nil; consumer-side PermissionHolder; archguard shrink-only allowlist)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (rela-security-reviewer run; see review RRs)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
