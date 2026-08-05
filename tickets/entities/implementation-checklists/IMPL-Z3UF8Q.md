---
id: IMPL-Z3UF8Q
type: implementation-checklist
title: 'Implementation: Relation field-level ACL redaction (visible:) — currently absent for relations, live and history'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (8 resolver + 9 handler tests)
- [x] Integration tests written (full HTTP handler flow via buildPolicyApp, not just units)
- [x] Happy path implemented (selective strip on live GET, all 4 read shapes)
- [x] Edge cases from planning handled (free-form key, empty role set, missing property, source-gone, deleted-endpoint)
- [x] Error handling in place — fail-closed everywhere; source-gone → empty meta, not a swallowed error

## Test Quality

- [x] Using fixture builders (seedEntity, buildPolicyApp, relHistoryStore, seedConditionalRelHistoryApp)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use the resolved meta map, not hardcoded strings

## Manual Verification

- [x] Feature verified end-to-end via handler tests (httptest through the real router handlers)
- [x] Each acceptance criterion verified with its planning test scenario
- [x] Edge cases verified

**Verification Evidence:** `go test ./internal/affordances/
./internal/dataentry/ ./internal/acl/` green (both default and `-tags postgres`
for the relevant packages). Live GET strips `secret`, keeps `reason`; incoming
resolves the source grant; history fails closed without reveal, shows meta with
`history:read-redacted`; restore preserves hidden meta (raw read); source-gone
fails closed. `just coverage-check` PASS (77.1% total).

## Quality

- [x] Code follows project patterns — optional interface type-asserted like TransitionResolver; capability mirrors entity `visible:` end-to-end
- [x] Checked for DRY — extracted shared `relationMetaStrip` + `buildRelationTypeRows` + `App.redactRelationMetaStrip` chokepoint (both live handlers route through it), sharpening the "every read shape redacts" contract
- [x] No security issues introduced — this IS a fail-closed ACL path; adversarial code review passed after fixes
- [x] No silent failures
- [x] No debug code left behind

**Quality gates:** `just lint` 0 issues, `just arch-lint` OK, `just fmt` clean.
