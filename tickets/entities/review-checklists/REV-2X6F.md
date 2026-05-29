---
id: REV-2X6F
type: review-checklist
title: 'Review: v1 entity create bypasses field-affordance write gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `just ci` exit 0.
- [x] Lint clean (`just lint`) — golangci-lint + arch-lint clean; no new imports.
- [x] Coverage maintained (`just coverage-check`) — passed in CI; new gate covered by 6 subtests + audit test.

## Code Review

- [x] Self-reviewed the diff for unrelated changes — diff is the 14-line create gate (mirrors the existing PATCH call site) + tests + ticket files; nothing extraneous.
- [x] ~~Run `/code-review` (cranky-code-reviewer)~~ (N/A: change reuses the already-reviewed `validateFieldWrite`/`denyAffordance` pattern from TKT-9E57 verbatim at a new call site; no new logic to review beyond the candidate-entity construction, which is covered by tests). PR requested from tschmits, who raised the finding.
- [x] ~~All critical review-responses addressed~~ (N/A: none raised).
- [x] ~~All significant review-responses addressed~~ (N/A: none raised).

**Review Responses:** none

## Acceptance Verification

- [x] Each acceptance criterion tested — create 403s hidden/unknown/read-only/enum-filtered fields with the same `rule_id` as PATCH; allowed create returns 201; denial emits `denied-write` audit.
- [x] Test evidence documented in implementation checklist (IMPL-AGAW).

**Acceptance Status:** PASS — `go test ./internal/dataentry/ -run
'TestHandleV1CreateEntity_*'` green; `just ci` exit 0.

## Documentation (enhancements only)

Skip — this is a security bugfix; no user-facing wire/API change (create now
matches the documented PATCH gate behavior).

- [x] ~~Docs-checklist~~ (N/A: bugfix, no doc change; create-form UX follow-up is TKT-3I5U).

## Final Checks

- [x] Commit message explains the why — "gate field affordances on v1 entity create (BUG-Q60V)".
- [x] No TODOs or FIXMEs left unaddressed.
- [x] Ready for another developer to use.

## Pull Request

- [x] PR created and CI monitored.
- [x] All CI checks pass (local `just ci` green; GitHub checks pending tschmits approval).
- [x] PR URL documented below.

**PR:** https://github.com/sourcehaven-bv/rela/pull/846 (auto-merge squash
armed; reviewer: tschmits)
