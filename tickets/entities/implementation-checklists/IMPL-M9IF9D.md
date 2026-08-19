---
id: IMPL-M9IF9D
type: implementation-checklist
title: 'Implementation: Gate the membership relation against ACL self-promotion'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`internal/acl/membership_gate_test.go`: 10-case predicate table + 8-case IsPrivileged table)
- [x] Integration tests written (`internal/appbuild/appbuild_membership_warn_test.go`: full `appbuild.New` boot on an on-disk temp project, slog captured)
- [x] Happy path implemented (predicate on `acl.Policy`, aclaudit delegation, `warnUngatedMembership` in `buildACL`, docs in both trees)
- [x] Edge cases from planning handled (configured/whitespace-padded membership relation, undeclared role, permission-only role, mixed assignments, no acl.yaml)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: pure boolean predicate + diagnostic log; no fallible operations added)

## Test Quality

- [x] Using fixture builders or factories for test data (reuses existing `writeMetamodel`/`writePolicy`/`appbuildOnDisk` helpers; shared `privileged`/`readOnly` RoleDef fixtures in the table test)
- [x] No hardcoded values in assertions when object is in scope (warn-test substrings — "member-of", "requires_permission" — ARE the warning's contract, asserted deliberately)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `cmd/rela` and ran against a temp project (2026-08-19):

- Ungated policy (`assignments: engineering: admin`, no `role_relations`
gate) → `rela list tickets` emits the WARN line naming `member-of`, the
`requires_permission` fix, `docs/acl-security.md`, and `rela acl audit`.
- `rela acl audit` on the same policy reports `[high] ... A1-ungated-membership: member-of`
— warning and audit agree (shared predicate).
- Gated policy (`role_relations.member-of.requires_permission: delegate-membership`)
→ no warning ("quiet as expected").
- Remaining planning ACs (read-only role, undeclared role, configured
relation name, no acl.yaml) covered by the automated tests listed above; all
pass via `go test ./...`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (the change IS a security control; the ticket's deferred item 4 explicitly NOT built — no latent `hasNonDefaultWorldGrant` hook exists)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no fallible paths added; predicate is pure, warning is best-effort diagnostics by design)
- [x] No debug code left behind

## Quality Gates (run 2026-08-19)

- `go build ./...` + `go test ./...` — all pass
- `just arch-lint` — OK, no warnings
- `just plimsoll` — clean
- `just lint` (golangci-lint) — 0 issues
- `just coverage-check` — package floor + total (76.6% >= 65%) satisfied
