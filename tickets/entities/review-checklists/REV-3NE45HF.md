---
id: REV-3NE45HF
type: review-checklist
title: 'Review: tenant: provision a new org — eager IdP webhook plus a single-flighted lazy backstop'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Automated Checks

- [x] All tests pass (`just test`): targeted tests pass on default, postgres (-race) and sqlite tags; full `go test -race` passed except sqlitestore, which timed out under load and passes alone
- [x] Lint clean (`just lint`); `just arch-lint` clean too
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (skipped: coverage thresholds were not run locally; CI runs the coverage gate on the PR)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-C3TADRN, RR-S7MEEO8, RR-QMGAGZF, RR-A1LYBQ2

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: webhook 200 only once the tenant exists
(TestHost_WebhookProvision). PASS: lazy 202 plus status polling
(TestHost_LazyProvision, TestHost_ProvisionStatus,
TestTenantMode_LazyProvision). PASS: single-flight
(TestAutoProvisioner_SingleFlight). PASS: erased org refused (provision postgres
test).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-postgres-backend, regenerated docs/postgres-backend.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-OJ746LJ

## Final Checks

- [x] Commit message explains the why, not just what (commit f2f2dac83; the user's commit-message convention is minimal)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: the PR is opened after the ticket is done, per TKT-UFV01M; not part of this bookkeeping)
