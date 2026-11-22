---
id: REV-9R4AHV3
type: review-checklist
title: 'Review: Tenant API for an external provisioning job'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./internal/tenanthost/ ./cmd/rela-server/, -race on TestTenantAPI; vet for every build tag; full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint --allow-serial-runners on the changed packages for each build tag)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (every handler branch has a test; CI runs the threshold check)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (self review plus a security review of the diff; this worker may not start agents)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-HCXIR3R)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-HCXIR3R, RR-ZPK8R6A, RR-SMBZHZM, RR-W06MC83, RR-E2K6KT4

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- PUT creates (201, 200, 202): PASS (TestTenantAPI_CreateThenRead, TestTenantAPI_SlowCreateIsAccepted)
- GET reads status or 404: PASS (TestTenantAPI_States, TestTenantAPI_CreateThenRead)
- Only a tenant API token for the org: PASS (TestTenantAPI_Auth, TestTenantAPIVerifier)
- Boot refusals: PASS (TestValidateTenantAPI, TestTenantAPI_Config, TestRefuseTenantOnlyFlags)
- Shared runner limits: PASS (TestTenantAPI_Failures)
- Documented: PASS (docs/postgres-backend.md and GUIDE-postgres-backend)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-1JRFUY9

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the owner's rule; the why is in TKT-84CD0PY)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (opened with gh pr create after pr-guidance-check)

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
