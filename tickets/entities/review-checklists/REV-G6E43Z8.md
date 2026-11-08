---
id: REV-G6E43Z8
type: review-checklist
title: 'Review: tenant: erase an org — DROP SCHEMA CASCADE, a documented retention SLA, and the audit-log residue'
started: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (race tests on the default and postgres builds for internal/tenant, internal/tenanthost, cmd/rela-server, internal/appbuild/backendtest and the dataentry webhook tests; the full `just test` runs in CI)
- [x] Lint clean (golangci-lint on the changed packages, default and postgres tags: 0 issues; arch-lint and plimsoll clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (new code is covered by the tests listed below; the floor check runs in CI)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer on the erasure diff)
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] All significant review-responses addressed (RR-7VW31EB, RR-FRPAC4U, RR-TS7HEYH, RR-QZT22A4, RR-Y92JSZ9)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7VW31EB, RR-FRPAC4U, RR-TS7HEYH, RR-QZT22A4,
RR-1BC0DZ0, RR-30V11TA, RR-I15PZXM, RR-JA14MOI, RR-JYZDFZ2, RR-I419PCJ,
RR-4M9O6BN, RR-EGU7HTA, RR-Y92JSZ9, RR-166LREN, RR-ZBI8HUE (all addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-HTZCASC)

**Acceptance Status:**

1. PASS: `TestConfig_MarkedTenantIsNotServed`, `TestConfig_MarkedTenantStillReservesItsSchema`.
2. PASS: `TestEraseTenant_Refusals`, `TestErase_RefusesBadInput`, `TestErase_RefusesSharedSchemas`.
3. PASS: `TestErase_RemovesOnlyThatTenant`, `TestErase_RefusesOpenDuringErasure`.
4. PASS: `TestErase_RemovesOnlyThatTenant`, `TestEraseTenant_EndToEnd`.
5. PASS: `TestErase_LostLockFailsClosed`, `TestErase_RefusesWhenPublicHasTables`.
6. PASS: GUIDE-postgres-backend, section "Erasing a tenant".

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-postgres-backend, regenerated docs/postgres-backend.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-TNERAS1

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the user's instructions; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist, see TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
