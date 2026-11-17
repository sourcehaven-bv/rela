---
id: IMPL-267YGV5
type: implementation-checklist
title: 'Implementation: Configure: edit flows.yaml triggers'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by TestConfigure_FlowsTrigger, which drives preview, save and the generation swap over HTTP; no browser walk-through was done)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test ./internal/configedit/ ./internal/appbuild/ ./cmd/rela-server/` passes.
TestCheckEdit_Flows covers allowed and locked trigger edits and reorders.
TestSave_FlowsOnly, TestSave_CreatesFlowsFile, TestSave_FlowsRefusals,
TestSave_UnchangedTriggerKeepsACLReference and
TestSave_PrepareFailureRestoresFlows cover save, audit, refusals and restore.
TestConfigure_FlowsTrigger shows a broken trigger is reported at preview and
refused at save with the file unchanged, and a valid edit serves a new
generation.
- Frontend: vitest 556 tests pass (models, configDraft rename, changes,
i18n guards); vue-tsc and eslint clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
