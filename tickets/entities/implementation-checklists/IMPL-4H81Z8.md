---
id: IMPL-4H81Z8
type: implementation-checklist
title: 'Implementation: List filter and sort evaluate hidden fields (value disclosure)'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (acl `TestConferrableRoles`, `TestFieldCeilingHides`; affordances `TestFieldVisibleOnEveryRow_*` including the differential against `FieldVerdicts`; dataentry `TestListFieldGate_SharedKeyParser`, `TestDemoResolver_FieldVisibleOnEveryRowMatchesVerdicts`, `TestListPushdown_Eligibility` virtual-key cases)
- [x] Integration tests written (test full flow, not just units) (`list_field_gate_test.go` drives the list handler per operator, sort, `when:`, granted control, `?q=`, and `scopedSortedEntities`; the golden corpus drives the production router)
- [x] Happy path implemented (`gateListQuery` in `listPage`, `scopedSortedEntities`, `navStatusRows`, `storePosition`; 400 `property_not_visible`)
- [x] Edge cases from planning handled (`when:` rejected; `id`/`modified` virtual keys skipped by the gate and declined by the pushdown; undeclared properties allowed; relation keys routed with the gate's classifier; malformed keys still 400 `invalid_filter`)
- [x] Error handling in place (errors surfaced, not swallowed) (`errPropertyNotVisible` mapped in `writeListPipelineError`; nav entries degrade per entry as before; list `?q=` maps a non-redacting searcher to `errACLListQuery`)

## Test Quality

- [x] Using fixture builders or factories for test data (`buildFieldPolicyApp`, `seedListGateTickets`, `newCeilingWorld`, `newStubLookup`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (golden corpus regenerated against the production router: every principal gets 400 `property_not_visible` for `filter[secret]`, `filter[secret][contains]` and `sort=secret`)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test -race ./internal/dataentry/ ./internal/acl/ ./internal/affordances/ ./internal/filter/ ./internal/appbuild/ ./internal/cli/ ./internal/mcp/`: all ok.
- `go vet -tags postgres` and `-tags sqlite` on dataentry: ok.
- `golangci-lint run --allow-parallel-runners` on dataentry, acl, affordances, filter: 0 issues.
- `just arch-lint`, `just comment-lint`, `just plimsoll`: pass.
- Golden: only the 21 renamed `list-filter-hidden*` / `list-sort-hidden*` files changed (200 to 400); no other recording moved.
- Before the fix `TestListFieldGate_HiddenPropertyRejected` returned 200 for every case, and the viewer's `?q=` returned a ticket whose only match was its hidden field.

## Quality

- [x] Code follows project patterns (check similar code) (consumer-side `fieldVisibility` interface; package functions, not App methods; search path reuses `searchVisibleHits`)
- [x] Checked for DRY opportunities (two filter-key parsers merged into `parseFilterKey`; virtual sort keys come from `filter.IsVirtualSortProperty`)
- [x] No security issues introduced (design reviewed by rela-security-reviewer; critical `?q=` finding fixed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
