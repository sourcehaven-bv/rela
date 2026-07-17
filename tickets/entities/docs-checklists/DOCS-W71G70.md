---
id: DOCS-W71G70
type: docs-checklist
title: 'Docs: missing-header detail in analyze view'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `MissingRequiredHeaders` (`internal/validation/content_rule.go`) — states it's a detail-only helper, NOT the pass/fail authority, and documents the two deliberate narrowings (pattern-check excluded, empty-match-string skipped) with rationale.
- [x] Godoc on `Violation.Detail` (validation.go), `RuleViolation` (validator.go), `AnalysisIssue.Detail` (analyze.go), `APIIssue.Detail` (settings_handlers.go), `AnalyzeIssue.detail` (config.ts) — each notes it's optional/per-violation and nil for non-content violations.
- [x] Comment on `IssuesTable.vue` `rowsFor` explaining the index-in-key collision guard (RR-D2BOYL).

## Project Documentation

- [x] ~~CLAUDE.md~~ (N/A: no new package, convention, or boundary — additive field threading.)
- [x] ~~README.md~~ (N/A: no project-level surface change.)

## User-facing Documentation

- [x] `docs/data-entry.md` — analyze-page section now documents the split click targets (entity title navigates; message reveals detail) and that a `content.required-headers` violation expands a detail row listing the missing exact headers (pattern checks excluded), plus the script-error-dialog path from the same message click.
- [x] ~~api-reference.md~~ (N/A: the `analyze_*` MCP tools / CLI surface is unchanged — detail is data-entry-view only per RR-1GP8NI; the `/api/v1/_analyze` REST endpoint's `APIIssue` gains an optional `detail` but that endpoint's field list is not exhaustively documented in api-reference.md.)
