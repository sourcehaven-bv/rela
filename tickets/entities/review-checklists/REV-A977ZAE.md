---
id: REV-A977ZAE
type: review-checklist
title: 'Review: AtomicWrites capability flag and capabilities.atomic_batch on _schema'
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

**Review Responses:** RR-2FB5KS6

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: `capabilities.atomic_batch` is false on fs (`TestV1Batch_UnsupportedStoreIs501`).
- PASS: it is true on sqlite (`TestV1Batch_SQLite_CapabilityIsAdvertised`).
- PASS: the schema goldens carry the field for every principal.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-QNHCEO0

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
