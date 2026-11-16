---
id: REV-GLJD81V
type: review-checklist
title: 'Review: Flows M5e: dogfood the tickets checklist workflow in tickets/flows.yaml'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -tags sqlite ./...`: 201 packages ok; the one
failure, `cmd/rela-desktop` `TestChromeStyle_TargetsShippedClasses`, needs a
built SPA this worktree lacks and is macOS-only; the branch touches no `cmd/`,
`frontend/` or `internal/dataentry` file. Postgres-tagged CI packages run
separately.)
- [x] Lint clean (golangci-lint on `internal/apptest`: 0 issues; plimsoll on
`internal/apptest`: clean)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained (`internal/apptest` 83.1%; no package floor change)

`just arch-lint`: OK, no warnings.

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and
rela-security-reviewer; the security reviewer had no findings)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-P75VLW5, RR-MK94XMB)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review RR-LSYNFLN, RR-JKEWK7W, RR-L3KGSTV,
RR-3K6BPS0, RR-YELRCA4, RR-5IIHOYI, RR-RSBI4YV, RR-Z3RUZUV, RR-ZOU8IWQ,
RR-DFFN21N; code review RR-P75VLW5, RR-MK94XMB, RR-CMJI3UA, RR-3ER51X9,
RR-FFGMA1J, RR-490KVE3, RR-1N919GH, RR-G93GON4, RR-11YOQ2T, RR-YDEIOI7,
RR-BCIXWAR.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: `rela test --require-tests` in `tickets/` runs 19 cases, one per
trigger plus edge cases, for tickets, bugs and checklists; CI runs it after
`rela flow validate`.
- PASS: `TestTicketsTriggersCovered` fails when a trigger has no case
(mutation check: renaming a case prefix failed it).
- PASS: CLAUDE.md carries the binary-version and create-in-status notes.
- PASS: DEC-617HJHB and the follow-ups TKT-QL15MAI, TKT-G33R2GW, TKT-IAQO7HG,
TKT-62NRDMJ and TKT-8WY8JAZ exist and are linked.
- PASS: the three analyze gates are clean.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: refactor
ticket)
- [x] GUIDE-flows points at the tickets example and says a property trigger
does not fire on create; `docs/flows.md` regenerated
- [x] ~~Docs-checklist marked as done~~ (N/A: refactor ticket)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is
opened after the ticket is done; GitHub records it, see TKT-UFV01M)
