---
id: IMPL-EYLHVJ
type: implementation-checklist
title: 'Implementation: acl audit A7 cannot see data-entry.yaml permission gates, so every UI-gating permission is reported dead'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`aclaudit.PermissionConsumer` declared in `aclaudit.go`; threaded through
`Audit` → `tierA` → `checkDeadPermissions`. The concrete `dataEntryPermissions`
adapter plus `loadDataEntryPermissions` live consumer-side in
`internal/cli/acl.go`, mirroring `MetamodelReader` / `metamodelReader`.

Integration coverage is the full `Audit()` flow plus end-to-end runs of the
built binary across five project shapes (built-in permission, UI-gated
permission, nested nav group, unparseable config, genuine typo).

Edge cases from planning, all handled: nested navigation groups (recursive
walk); missing `data-entry.yaml` (complete information → empty consumer, NOT
nil); unparseable config (incomplete information → nil → A7 suppressed).

Errors are surfaced, not swallowed: `loadDataEntryPermissions` returns its
error, and the caller emits an operator-visible warning naming the file and the
parse error before suppressing the check.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Added a `writeDataEntry(t, root, body)` helper (with `t.Helper()`) so each
per-surface case supplies only the YAML fragment that matters — a document, a
card, a nav entry, a nested nav entry, a command — and nothing else.

Adapter tests assert on the permission constant in scope rather than a repeated
literal. Each surface is its own subtest, named for the surface, so a failure
identifies which gate the adapter dropped.

Two `PermissionConsumer` test doubles instead of ad-hoc closures: `allPerms`
(references nothing — used by non-A7 tests to opt into the check running) and
`usedPerms` (a fixed set).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran the built binary against temp projects covering each acceptance criterion:

1. Permission granted, no `data-entry.yaml` → correctly reported dead (nothing
references it).
2. Same permission gating a document → `✓ ACL audit: no findings.`
3. Same permission on a **nested** nav entry under a `group:` → clean (proves
the recursive walk).
4. Unparseable `data-entry.yaml` → warning emitted AND no dead-permission
finding.
5. Typo'd permission alongside a live UI-gated one → only the typo reported.

**A real defect was caught here that unit tests missed.** The first wiring did
`perms, err := load(...)` then `perms = nil` on error — assigning a nil
`*dataEntryPermissions` into an interface, producing a non-nil interface holding
a nil pointer. `Audit`'s `perms == nil` never fired, so scenario 4 printed the
"skipping" warning and then reported the finding anyway. Fixed by declaring
`perms` at interface type; pinned by `TestAudit_A7_TypedNilConsumerIsAnAnswer`
and a comment at the declaration explaining why the type matters.

Per-surface coverage was mutation-tested: deleting the nav recursion fails only
the nested-navigation subtest; deleting the dashboard loop fails only the
dashboard subtest. Each surface is independently guarded.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the consumer-side-interface rule in CLAUDE.md and the concrete precedent
in this very file: narrow interface declared where it is consumed, adapter
supplied at the wiring site. `just arch-lint` confirms aclaudit still depends
only on `internal/acl` — no `dataentryconfig` import crossed the boundary.

DRY: `navigationPermissions` is recursive rather than a duplicated loop for
groups; `writeDataEntry` removes repeated temp-file setup across five subtests.

Security: the change only ADDS to a used-set — it can suppress an advisory
finding, never grant access. The adapter deliberately does NOT validate the
config, so an invalid `data-entry.yaml` cannot cost an operator their ACL
findings; that is a robustness choice, not a security relaxation. Note the
config surfaces read here are operator-authored and non-secret per CLAUDE.md
("the configuration is not a secret; the data is"), so reading permission names
discloses nothing.

No silent failures: the one failure path (unreadable/unparseable config) both
warns the operator and changes behaviour conservatively (suppress rather than
assert). `gofmt`, `golangci-lint` (0 issues), `just arch-lint`, and `go test
-race` on all three touched packages clean.
