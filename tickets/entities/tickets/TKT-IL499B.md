---
id: TKT-IL499B
type: ticket
title: 'Analysis view: reveal which detail failed a validation (missing headers etc.) on hover/click'
kind: enhancement
priority: medium
effort: m
status: review
---

In the data-entry Analysis view, the Validations table shows a flat rule
`Description` as the message (e.g. "NCR body moet standaardkoppen bevatten").
The user cannot tell **which** detail failed — e.g. which required headers are
missing. Add a way (hover tooltip and/or click-to-expand) to reveal the specific
failing detail per validation issue.

## Problem

Content-rule violations carry only the static `rule.Description`. The identity
of the missing header(s) is computed in `internal/validation/content_rule.go`
`CheckContentRule` and immediately discarded (function returns a plain `bool`).
Nothing between the check and the Vue table has a place to carry per-issue
detail.

## Scope

Surface the specific failing detail for content-rule (required-headers)
violations, rendered as a hover/expand affordance in the MESSAGE cell of
`AnalyzeView.vue`.

Detail must thread through the layers that are currently detail-free:
1. `internal/validation/content_rule.go` — `CheckContentRule` returns the failing header(s) instead of just `bool`.
2. `internal/validation/validation.go` — `Violation` / `newViolation` carry an optional detail.
3. `internal/validator/validator.go` — `RuleResult.Violations` preserve detail (currently `[]string` of IDs only).
4. `internal/dataentry/analyze.go` — `AnalysisIssue` gains a detail field; `analyzeValidations` populates it.
5. Wire: `internal/dataentry/settings_handlers.go` `APIIssue` + `internal/dataentry/api_v1.go` `handleV1Analyze` mapping.
6. Frontend: `frontend/src/types/config.ts` `AnalyzeIssue`, and `AnalyzeView.vue` MESSAGE cell (line ~240) rendering.

The existing `scriptError` envelope is the precedent for carrying structured
per-issue detail end-to-end (but it is Lua-failure-only today).
