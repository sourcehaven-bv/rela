---
id: REV-G46QQD
type: review-checklist
title: 'Review: Conversion dialog disappears on Review'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Tests pass: `go test ./internal/dataentry/`, frontend `npm run test:run`, full e2e suite
- [x] Lint clean: frontend `npm run lint` (no new errors), `npm run typecheck`, e2e eslint, gofmt

## Code Review

- [x] Self-reviewed the diff for unrelated changes

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: Review keeps the dialog open and shows the review step.
- PASS: Convert after Review creates the target entity and navigates to it.
- PASS: a server error on preflight is shown in the dialog.

## Final Checks

- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use
