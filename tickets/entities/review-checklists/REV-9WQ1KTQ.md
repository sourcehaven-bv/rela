---
id: REV-9WQ1KTQ
type: review-checklist
title: 'Review: Help texts for every form placement'
started: "2026-11-07"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): frontend `npm run test:run` 5993 pass; Go `go test -race ./...` passed on the first commit, and `internal/dataentryconfig` passes after the follow-up; CI runs the full Go suite
- [x] Lint clean (`just lint`): golangci-lint 0 issues on all four build tags; frontend lint 0 errors
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the Go change adds struct fields only, with a round-trip test; the CI coverage job enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CI3NWY1 (significant, addressed), RR-UA47KDH,
RR-ZO5QCVU, RR-C32WDER, RR-W31L5TS, RR-LH7AVSP, RR-OX9ZXHG, RR-KE8E693,
RR-JFCG7V6 (minor, addressed), RR-UTXMHH8 (minor, wont-fix), RR-VATPDVH,
RR-3P9VWBH (nit, addressed), RR-OGQ74K9 (nit, wont-fix). The security reviewer
found nothing.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: `TestFormHelpKeys_YAMLAndJSON`.
2. PASS: `FieldShell.test.ts` hint toggle, aria and label-less name; checked in the browser.
3. PASS: `RelationPicker.test.ts`, `RelationCards.test.ts` (including the description fallback); checked in the browser.
4. PASS: `DynamicForm.help.test.ts` (including details closing on a step change); checked in the browser.
5. PASS: `MarkdownHelp.test.ts` under jsdom (script, onerror, javascript: link).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-KJ222ZM

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
