---
id: IMPL-D141K1
type: implementation-checklist
title: 'Implementation: acl audit A7 reports rela''s own built-in permissions (history:read) as dead, and suggests a fix that breaks a working grant'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`acl.BuiltinPermissions()` added in `internal/acl/policy.go` beside the
constants it enumerates; `checkDeadPermissions` seeds `used` from it.

Integration coverage is the full `Audit()` flow (policy → tierA → finding set),
not a direct call to the unexported check — plus an end-to-end run of the built
`rela acl audit` binary against a real project directory, which is what caught
the sibling bug's typed-nil defect.

Edge case from planning: a built-in must not blanket-suppress A7. Covered by
`TestAudit_A7_BuiltinDoesNotMaskRealDeadPermission`.

No new error paths — `BuiltinPermissions()` is a pure constructor over constants
and cannot fail.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The load-bearing property here: the test iterates `acl.BuiltinPermissions()`
rather than a hardcoded `{"history:read", "history:read-redacted"}` literal.
That is what makes a newly added global constant fail the test instead of
silently becoming a false "dead" report — the why5 failure mode. Subtests are
named from the permission value in scope, and assertions reference `perm`, not a
repeated string.

Policies are built inline with only the fields each case needs (a role, its
`permissions:` list). The package has no fixture-builder convention; matching
the surrounding table-driven style was the right call over introducing one.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the binary and ran `rela acl audit` against a temp project (the exact
reproduction from the bug report).

With the fix:

```
✓ ACL audit: no findings.
```

With the seeding loop reverted (same project, same command):

```
⚠ ACL audit: 1 finding(s)
  [low] role "admin" grants permission "history:read" which no
        role_relations.requires_permission references; the permission is dead
        fix: reference "history:read" in a requires_permission gate, or remove it
```

The A/B confirms the fix addresses the reported behaviour and not an adjacent
one. Unit-level equivalent: reverting the loop fails all three new subtests
(`history:read`, `history:read-redacted`, and the mask guard) with the finding
text quoted verbatim.

Edge case verified manually: a typo'd permission granted alongside a built-in is
still reported dead.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the existing pattern: the exported accessor lives in `internal/acl` next
to its constants, so aclaudit needs no new dependency.

DRY: `BuiltinPermissions()` IS the de-duplication — it replaces what would
otherwise be a literal list repeated in every consumer that needs to know which
permissions rela ships. Deliberately not extracted further; the seeding loop is
three lines and factoring it would obscure the contract.

Security: the change only ever ADDS to a used-set, so it can suppress a finding
but never grant access. A7 is advisory and touches no evaluation path. Verified
A6 is unaffected — `permissionGranted` asks the reverse question.

`gofmt`, `golangci-lint` (0 issues), `just arch-lint` (no warnings), and `go
test -race` on `internal/acl`, `internal/aclaudit`, `internal/cli` all clean.
