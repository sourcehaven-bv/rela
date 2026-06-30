---
id: IMPL-ER8TOC
type: implementation-checklist
title: 'Implementation: ACL: make the membership relation (member-of) configurable via membership_relation: in acl.yaml'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (resolver walks via Declarative.ForPrincipal → Globals, the real path, not a mocked unit)
- [x] Happy path implemented
- [x] Edge cases from planning handled (blank/whitespace → default; non-default not following member-of)
- [x] Error handling in place (errors surfaced, not swallowed; warnings via slog, walk-abort behaviour unchanged)

## Test Quality

- [x] Using the existing fakeGraph fixture + newTestDeclarative helper
- [x] No hardcoded values in assertions when object is in scope (assert against constructed RoleAttribution/Source values)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature exercised end-to-end through the resolver
- [x] Each acceptance criterion verified with the planned test scenario
- [x] Edge cases manually verified (whitespace table; match-all guard)

**Verification Evidence:**

Code (all in `internal/acl/`):
- `policy.go`: `Policy.MembershipRelation` field + `membership_relation` yaml tag;
`defaultMembershipRelation` const; non-mutating `membershipRelation()` accessor
(isBlank → default); `membership_relation` in `knownPolicyKeys`; two advisory
`slog.Warn` hardening checks in `warnMembershipRelationHardening()` (called from
Validate, gated on effective != default); godoc on `Policy` + `RoleRelationDef`.
- `resolver.go:65`: walks `r.d.policy.membershipRelation()`; docstrings updated.

Tests added (AC → test):
- AC1 → `TestMembershipRelation_Configured_ConfersGroupRole` (heeft_rol edge →
editor with Source{SourceGroup,"engineering"}; also asserts the walk queried
heeft_rol and NOT member-of).
- AC2 → `TestMembershipRelation_Default_WhenUnset` (blank → member-of walked).
- AC3 → `TestMembershipRelation_Configured_DoesNotFollowMemberOf` (negative: a
member-of edge is not followed; asserts no Type=="" match-all query issued).
- AC4 → `TestMembershipRelation_Configured_Transitive` (A→B→C heeft_rol chain).
- AC5 → `TestPolicy_MembershipRelation_UngatedWarns` +
`TestPolicy_MembershipRelation_GatedNoWarn` (warning fires un-gated, silent when
gated; Validate returns nil either way).
- AC6 → `TestPolicy_membershipRelation_EffectiveName` (table: ""/spaces/tab/
newline/explicit-default → member-of; "heeft_rol" → heeft_rol).

Gate results (all green):
- `go build ./...` — OK
- `go test -race ./internal/acl/` — ok; new tests confirmed run via `-v`
- `go test ./internal/dataentry/... ./internal/mcp/... ./internal/appbuild/...` — ok
- `golangci-lint run ./internal/acl/` — 0 issues
- `just arch-lint` — no warnings
- `just plimsoll` — rc=0 (Policy gained 2 methods, far under the 40 cap)
- `just coverage-check` — PASS (acl 88.5%; floors satisfied)

Backwards-compat (AC "behaves identically to today"): all pre-existing
resolver_test/features_test cases pass unchanged (they build default policies);
the only production literal "member-of" is now the single shared const.

## Quality

- [x] Code follows project patterns (mirrors InheritRolesThrough accessor/walk
pattern; warning style matches existing unknown-key slog.Warn)
- [x] Checked for DRY opportunities — single `defaultMembershipRelation` const +
one accessor is the single source of truth; reused existing `isBlank`
- [x] No security issues introduced — closed the blank-relation match-all
over-grant (design-review RR-LFMR7S/WRFCW5); added escalation foot-gun warning
- [x] No silent failures (warnings emitted; walk-abort semantics unchanged)
- [x] No debug code left behind (the accidental declarative.go scratch edit was
reverted; git diff confirms only intended files changed)
