---
id: IMPL-TRSW3W
type: implementation-checklist
title: 'Implementation: Record the archplan baseline and run archplan in CI'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new Go logic; the change adds guard markers, a glossary, workflows and the baseline)
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests added)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `ARCHPLAN_CI_TRUST=1 just archplan-security` with archplan v26.10.30:
82 traced diagrams, 416 findings. Against the proposal, `unchecked_entry` on
handleV1Comments, `unguarded_path` rollup -> store and `unguarded_entry` on
handleV1Conflicts (#2011) are gone. New since the proposal: 10 `untraced` (the
five comment guards, document gates, schema and conflict sub-routes, live PUT)
and 1 `repeated_orchestration` with permitsGitSync. None is a defect: the routes
suite does not reach them.
- Baseline: 412 accepted, every entry with a group reason, none "baseline".
The 5 entity_search_* orphan tables stay out; explicit schema.owners entries did
not clear them (owner is already sqlitestore; uses = 0).
- CI simulation without traces or hotspots, base exported separately and
passed with --base-model: "Passed: 0 new findings, 0 ratchet metrics worse"; the
5 orphan tables are "already on the base".
- actionlint: no new findings in either workflow (only the existing
ubuntu-26.04 runner-label notice).
- go build ./..., go test ./internal/dataentry -run Comment,
./internal/archguard, ./internal/rollup, just arch-lint, comment-lint gate,
golangci-lint on the touched packages, go test ./internal/dataentry: clean.
- Weekly report: jq filter run on a real traced `archplan ci --json` (0 new
security findings, 0 worse metrics), and the issue body rendered from a sample
report with 3 findings and 1 worse metric.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
