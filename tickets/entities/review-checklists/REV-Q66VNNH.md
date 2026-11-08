---
id: REV-Q66VNNH
type: review-checklist
title: 'Review: tenant: scheduler, CLI and stdio MCP in a multi-tenant deployment'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on internal/tenant, internal/cli, cmd/rela-server and cmd/rela-docs, default and postgres tags; full suite under `just coverage-check`. Four `TestLeaseCLI_*` tests fail on the postgres tag locally without `RELA_DATABASE_URL`, identically on origin/develop
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, default and postgres tags, 0 issues; `just arch-lint` and `just plimsoll` clean
- [x] Comment lint gate clean (`just comment-lint`); no advisory finding introduced
- [x] Coverage maintained (`just coverage-check`)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent), plus rela-security-reviewer
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-4APOTQ6, RR-4VENM5P, RR-SG4SA74 (significant,
addressed); RR-DCIIUHX, RR-AH01SNI, RR-7SB9IM5, RR-V0SLNVF, RR-EZ8OXLN (minor,
addressed); RR-C43C233 (minor, deferred); RR-ULFOGMP (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: TestTenantScheduler_WritesOnlyIntoItsTenant
- AC2 PASS: TestCheckTenantMode, TestCheckTenantMode_EveryCommandIsGated, TestOpenProject_TenantModeFailsClosed, TestNewMCPServices_TenantModeFailsClosed, TestTenantMCP_SeesOnlyItsTenant
- AC3 PASS: `--tenant` takes an org id only; TestTenantOpen_ErrorNamesNoPassword

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VIVAYBL

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist; see TKT-UFV01M)

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
