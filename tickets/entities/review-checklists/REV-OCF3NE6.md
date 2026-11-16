---
id: REV-OCF3NE6
type: review-checklist
title: 'Review: Flows M5d: entity-change triggers in flows.yaml and delete internal/automation'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (every package passes, with -race and -shuffle in the coverage run; cmd/rela-desktop TestChromeStyle_TargetsShippedClasses needs the built frontend, which this worktree lacks, and runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on every changed package: 0 issues; arch-lint, plimsoll and go vet with the postgres, sqlite and memorybackend tags pass)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (go-test-coverage on the profile: total 85.8%, every package floor met)

**Comment findings.** `just comment-report` lists no finding in the new
triggers, migration, appbuild or CLI code.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-BZX4ZZM, RR-G3QH9AN (significant, addressed);
RR-E43GKVM, RR-U7358SX, RR-T9AXW6N, RR-Z1MYL9O, RR-C7006RO, RR-EIKS2LR,
RR-E3SQ1AS, RR-92O6L6K (minor, addressed); RR-Z3M5OFQ, RR-44NG9VZ, RR-RO5MWTU,
RR-14Y1PQR (nit, addressed); RR-8IBU441 (nit, deferred to TKT-ZT2KREN). The
rela-security-reviewer found one minor item (RR-92O6L6K) and nothing critical or
significant.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- `internal/automation` has no callers and is removed: PASS. The package is
deleted; the build, arch-lint and the archguard and agentrules path checks pass
with internal/flowcfg/triggers in its place.
- The M5b goldens pass against the new path: PASS. The synthetic, tickets,
basecamp and dataentry goldens pass; only engine message texts changed in
synthetic.jsonl. docs-project declares no automations, so it has nothing to
migrate.
- The migration converts every in-repo project and is idempotent: PASS. Five
projects were converted with `rela migrate config` and pass `rela validate`;
TestMigrate_CrashRecovery re-runs after a crash at each write. Moving this
ticket to review created REV-OCF3NE6 through the migrated tickets/flows.yaml
trigger.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: kind is refactor)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: kind is refactor)

**Docs Checklist:** N/A. GUIDE-flows has the Triggers section and the migration
table; GUIDE-metamodel and the other guides point at it; docs/ and README.md are
regenerated.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened by hand after this ticket is done, as the brief asks)

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
