---
id: REV-6UULOC
type: review-checklist
title: 'Review: Attachment web read path: ACL-gated download endpoint + file widget/preview'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./internal/dataentry/ ./internal/store/...` green; frontend `npm run test:run` 1082 passed
- [x] Lint clean — `golangci-lint run ./internal/dataentry/` 0 issues; frontend `npm run lint` 0 errors (warnings all pre-existing, none in changed files); `vue-tsc` typecheck clean; `just arch-lint` OK
- [x] Coverage maintained — `just coverage-check` PASS; dataentry 77.8%

## Code Review

- [x] Ran code review (cranky-code-reviewer agent) — thorough, security-focused
- [x] All critical review-responses addressed — RR-FKJYMC (addressed)
- [x] All significant review-responses addressed — RR-SQE8VA, RR-XB5738 (addressed)
- [x] Self-reviewed the diff for unrelated changes — scoped to the read-path; the shared `entityNotFoundTitle` hoist touches gate/entity-GET 404 literals (intentional, pins the indistinguishability invariant)

**Review Responses:** RR-FKJYMC (critical), RR-SQE8VA (significant), RR-XB5738
(significant), RR-601TJD (minor) — all `addressed`.

## Acceptance Verification

- [x] Each acceptance criterion tested:
  - AC1 allowed→bytes+headers: `TestAttachment_AllowedReturnsBytes` PASS
  - AC2 denied→404 parity, no ETag, matches gate body: `TestAttachment_DeniedIsNotFound` + `TestAttachment_DenyBodyMatchesGate` PASS
  - AC3 missing/unknown/non-file→404: `TestAttachment_UnknownOrNonFileProperty` PASS
  - AC4 rename resolves by current id: `TestAttachment_ResolvesByCurrentIDAfterRename` PASS
  - AC5 SPA preview/download + edit-mode read-only: FileWidget tests PASS
  - XSS defenses: `TestAttachment_DeceptiveExtensionServesTypeFromName` + `TestAttachment_SvgIsSandboxed` PASS
  - `_attachments` on per-entity GET + mutation, absent on list rows: `TestAttachment_MetadataOnEntityGET` + `TestAttachment_MetadataOnMutationResponse` PASS
- [x] Test evidence documented (implementation checklist + above)

**Acceptance Status:** ALL PASS

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — DOCS-KU30G9
- [x] User-facing documentation updated — `docs/data-entry/api-reference.md` Attachments section
- [x] Docs-checklist marked as done — DOCS-KU30G9 `done`

**Docs Checklist:** DOCS-KU30G9

## Final Checks

- [x] Commit message explains the why — pending commit (user controls timing)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr`~~ (deferred: user controls commit/PR timing; changes staged in working tree)
- [x] ~~CI checks pass~~ (deferred to PR — full local CI surrogate green: build default+postgres, lint, tests, coverage, arch-lint)
- [x] ~~PR URL~~ (deferred)

**PR:** pending — changes staged, not yet committed/pushed
