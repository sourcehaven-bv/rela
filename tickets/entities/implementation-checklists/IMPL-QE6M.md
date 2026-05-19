---
id: IMPL-QE6M
type: implementation-checklist
title: 'Implementation: ACL v0 PR 2: Declarative ACL + Policy loading (acl.yaml)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A for PR 2: Declarative is not wired into production until PR 3; integration with Manager comes there)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

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

- **`internal/acl/policy.go`**: typed Policy + LoadPolicy. Decodes via `gopkg.in/yaml.v3` with a manual first-pass for unknown-key warnings (warn-not-fail; matches the metamodel loader's tolerance).
- **`internal/acl/declarative.go`**: Declarative ACL. `effectiveRoles` returns explicit-assignment + default (deduped, stable order). `holdsPermission` is a union scan. `AuthorizeWrite` runs delegate-X first (only when RelationType matches a `RoleRelations` entry with `RequiresPermission`), then type-level write (wildcard `*` or exact match), then deny with the documented Reason string.
- **`internal/acl/policy_test.go`** — verifies AC2.1: Empty / FullExample / UnknownKey_LogsWarning / MissingFile_ReturnsErrNotExist / MalformedYAML_ReturnsParseError. UnknownKey test captures `slog.Default()` output via a `bytes.Buffer` handler.
- **`internal/acl/declarative_test.go`** — verifies AC2.2–AC2.7 across realistic scenarios: contributor allowed on ticket; reviewer denied on ticket with structured reason; admin (`*`) on any type; alice (no `delegate-contributor`) denied on `ticket-owner` relation; admin allowed on same; relation without `requires_permission` skips delegate gate; unknown principal falls through to `default`; multi-role principal gets union with explicit role winning RuleID attribution. Plus negatives: undefined-role assignment drops to default; empty WriteRequest decays to "no role grants write on type 'relation'"; Decision converts cleanly to `*acl.ForbiddenError`.
- **All tests** in `internal/acl/` pass: 6 from PR 1 (NopACL × 8 subtests, ReadOnlyACL × 7, ForbiddenError × 3) + 5 from PR 2 policy + 11 from PR 2 declarative.
- **`just ci` exits 0** end-to-end on `feat/acl-v0-pr2` (branched off `feat/acl-v0`).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
