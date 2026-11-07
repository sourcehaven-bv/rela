---
id: REV-26RPTZG
type: review-checklist
title: 'Review: rela test v1 (1/4): untagged in-memory appbuild recipe with MemFS write / disk config split'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (full race+coverage suite passes except dataentry and sqlitestore, which hit the 10m timeout at load average 30; both re-run without race and pass)
- [x] Lint clean (`just lint`) (golangci-lint on internal/appbuild and internal/storage: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (go-test-coverage: package and total thresholds PASS)

Also clean: `just arch-lint`, `just plimsoll`, `go vet` on default,
memorybackend, postgres and sqlite tags.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer run on the diff)
- [x] All critical review-responses addressed (RR-8ULC8I2 deferred to TKT-NRNDGZR, which owns the output dir)
- [x] All significant review-responses addressed (RR-5PV25TX deferred to TKT-NRNDGZR; the rest fixed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-8ULC8I2, RR-5PV25TX, RR-Z7SEMB6, RR-Y197PJC,
RR-D0R3LJS, RR-X3V6OMC, RR-TI5T9S8, RR-XKFHWA0, RR-P4KWF50, RR-387UB5O,
RR-BJAS6Q1, RR-485BI9F, RR-OUMEUWO, RR-COIHZ2X, RR-02RIYX1, RR-1RU6FFB,
RR-VJ2V8EN

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-FEHVM7J)

**Acceptance Status:**

- Untagged NewInMemory over memstore + LinearSearch, MemFS writes, disk config: PASS (memrecipe.go; memorybackend New delegates).
- Project tree hash unchanged on a project with comments, piles and a pending migration: PASS (TestNewInMemory_LeavesProjectTreeUntouched).
- goleak over 100 build+Close cycles: PASS, with the neoq cron runner exempt and counted (TKT-L98KKVX).
- Automations, validation and ACL run on writes: PASS (TestNewInMemory_WritesRunTheWritePath).
- CI dependency gate covers the recipe: PASS (postgres build of internal/appbuild links memstore and no bleve; gate now fails on a match).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal wiring; no user-facing change until rela test lands)
- [x] ~~User-facing documentation updated~~ (N/A: internal wiring)
- [x] ~~Docs-checklist marked as done~~ (N/A: internal wiring)

## Final Checks

- [x] Commit message explains the why, not just what (minimal one-line message per the brief)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh pr create after done; the parent session monitors CI)
