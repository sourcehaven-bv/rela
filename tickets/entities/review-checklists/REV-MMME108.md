---
id: REV-MMME108
type: review-checklist
title: 'Review: tenant: assemble every tenant over one SharedBase so config is parsed once'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (ran the affected packages instead of the full suite: `go test -race` on internal/appbuild/... and internal/tenant on the default and postgres builds, `-tags sqlite` and `-tags memorybackend` on internal/appbuild; CI runs the rest)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages for all four build tags: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: new code is covered by new tests; CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7F96BSE, RR-QGDBMLG (significant, addressed);
RR-ANDDXA0, RR-14DHIM8 (minor, addressed); RR-V3CSVT4, RR-WOQG2MQ (minor,
deferred); RR-G5FV121 (nit, addressed); RR-J4B6NQH, RR-2KNCPTG (nit, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestTenantIsolation_OneTenantCannotReadAnother` asserts shared metamodel and ACL policy pointers across two leases, and fails when each tenant re-prepares.
2. PASS: `TestSharedBase_AssemblyDoesNotMutateSharedValues` and the postgres isolation suite pass.
3. PASS: the `Registry` and `Opener` docs now say the shared config comes from `SharedBaseOpener`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] ~~User-facing documentation updated~~ (N/A: internal wiring, no user-facing change)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-08W7LX3

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
