---
id: IMPL-6R4CXE
type: implementation-checklist
title: 'Implementation: Analysis view: reveal which detail failed a validation (missing headers etc.) on hover/click'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`content_rule_test.go` `TestMissingRequiredHeaders`; `validator_test.go` `TestGenericValidator_CheckRuleFull_ContentDetail`)
- [x] Integration tests written (`validation_test.go` `TestContentViolationCarriesMissingHeaderDetail`; `analyze_test.go` `TestAnalyzeValidations_AttachesMissingHeaderDetail`; `api_v1_test.go` `TestV1Analyze_ContentDetailPassthrough` — store→validator→analyze→API round-trip)
- [x] Happy path implemented (missing exact headers surfaced end-to-end)
- [x] Edge cases from planning handled (pattern-check excluded, empty match-string skipped, nil-when-none, per-violation detail)
- [x] Error handling in place (no new error paths; detail is additive/omitempty)

## Test Quality

- [x] Using fixture builders (`makeIssue`/`makeResult` in `AnalyzeView.test.ts`; table tests in Go)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran the real data-entry server (`rela-server`) against a throwaway project with
an NCR entity-type + `content.required-headers` rule (`## Beschrijving`, `##
Oorzaak`, `## Corrigerende maatregel`) and an NCR-001 entity that has only `##
Beschrijving`.

- **API (curl `/api/v1/_analyze`)** returned the Validations issue with
`"detail": ["## Oorzaak", "## Corrigerende maatregel"]` — the two genuinely
missing headers, omitting the present one. omitempty confirmed (field only
present when populated). This exercises the full store→validator→analyze→wire
chain including the RR-UFUX7T loop rewrite.
- **AC1 (expand)** — in the browser, the Validations row showed the flat message
with a ▸ disclosure; clicking the message expanded a full-width detail row
"Missing required headers: ## Oorzaak / ## Corrigerende maatregel" in monospace.
Screenshot captured; matches the reported use case.
- **AC1 (collapse)** — second click collapsed the detail row (`aria-expanded`
toggled false, detail row removed from DOM).
- **AC2 (split targets)** — clicking the entity title routed to
`/entity/ncr/NCR-001`; the message click did not navigate. Confirmed the row is
no longer a single click target.
- **CLI scope boundary (RR-1GP8NI)** — `rela analyze validations` still shows only
the flat description (no detail), as intended.

Automated: `go test ./internal/...` all pass (incl. validation, validator,
dataentry, analysis, cli, mcp — the RuleResult blast-radius consumers). Frontend
`npm run test:run` 1181 pass (11 existing AnalyzeView tests updated for the new
split-target interaction + 4 new detail-row tests). `just arch-lint` clean,
`golangci-lint` 0 issues on changed packages, `vue-tsc` + eslint clean on
`AnalyzeView.vue` / `IssuesTable.vue`.

## Quality

- [x] Code follows project patterns (reused the `description` override slot; detail threads the same way `scriptError` does)
- [x] Checked for DRY opportunities — extracted the issues table into `IssuesTable.vue` (kept `AnalyzeView.vue` under the `max-lines` god-component cap and made the table independently testable/reusable)
- [x] No security issues introduced (detail is metamodel-authored, escaped `{{ }}`/`:aria-expanded`, no `v-html`; ACL-filtered upstream)
- [x] No silent failures
- [x] No debug code left behind
