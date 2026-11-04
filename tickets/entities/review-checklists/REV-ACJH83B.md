---
id: REV-ACJH83B
type: review-checklist
title: 'Review: POST /api/v1/_batch endpoint, OpenAPI and API reference'
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

**Review Responses:** RR-6VCPH64, RR-8W6704I, RR-CRIOYOZ, RR-CRVAETI,
RR-DQA0YQ6, RR-E3IPT8D, RR-E758LR1, RR-I2B6GIB, RR-IHWNWQQ, RR-KQRQXCF,
RR-MNYZCY0, RR-QWP30LP, RR-RLMZYQY, RR-SL3ADIC, RR-U6ZJ7SK, RR-VWDSGMF,
RR-W5TVRRE, RR-WJS1O9V

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: 501 `atomic_batch_unsupported` on fs and memory.
- PASS: limits of 100 operations (400) and 1 MiB (413) (`TestV1Batch_RequestLimits`).
- PASS: shape errors name a JSON pointer (`TestV1Batch_ShapeErrorsNamePointer`).
- PASS: ACL denials write nothing and answer 403 with operation and rule (`ACLDenialNamesOperation`, `FieldGateDenialWritesNothing`).
- PASS: a hidden end or source answers the same as a missing one.
- PASS: an unreadable create is redacted (`UnreadableCreateIsRedacted`).
- PASS: OpenAPI path documented (`TestBatchPath`); api-reference and ACL guide updated.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-8TRAFD9

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
