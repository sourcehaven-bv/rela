---
id: REV-54IAH6F
type: review-checklist
title: 'Review: Atomic batch engine: entitymanager.CreateBatch'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Evidence: `go test -race` passes on entitymanager, dataentry, openapi, apiwire,
store and sqlitestore; sqlite and postgres tagged runs pass; golangci-lint
reports 0 issues; plimsoll, arch-lint and comment-lint pass; `just
coverage-check` passes.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Reviews: cranky-code-reviewer and rela-security-reviewer. The security review
found no critical or significant issues.

**Review Responses:** RR-2EGZ18C, RR-3LT4WBL, RR-65I53VC, RR-734XHMA,
RR-9L87EIA, RR-FKVECPU, RR-KCUJVQW, RR-RLSQHTM, RR-RVPDJ0L, RR-SE3ULLG,
RR-T1M860X, RR-Z62OFQK

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: a batch creates every entity and relation in one `store.Tx` (`TestCreateBatch_*` on memory, sqlite and postgres).
- PASS: any failure on a rollback store leaves no rows (sqlite `ConflictRollsBack`, postgres rollback subtests).
- PASS: on fs/memory a failure after a write returns `BatchKeptError` and audits the kept rows.
- PASS: ends are rechecked inside the Tx; a missing end or face fails the batch.
- PASS: automations run after commit (decision 2).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RH70RRV

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
