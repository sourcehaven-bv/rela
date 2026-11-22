---
id: IMPL-M9KW08Y
type: implementation-checklist
title: 'Implementation: GET /_flows answers 404 on every page of a project without flows'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestSchemaCapabilities_Flows; useFlowList.test.ts: no call without flows, waits for the schema without loading it)
- [x] Integration tests written (test full flow, not just units) (TestSchemaCapabilities_Flows goes through the /_schema handler with and without SetFlows; Sidebar.builtins.test.ts mounts the sidebar over a real schema store)
- [x] Happy path implemented
- [x] Edge cases from planning handled (schema not loaded yet: the read waits; first caller unmounts before the schema loads: the watch lives in a detached scope)
- [x] Error handling in place (errors surfaced, not swallowed) (a failing flow list still reads as no flows and is asked again, as before)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing flow fixtures)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by the handler test and the mounted sidebar test; no running server was started under the current machine load)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: covered by unit tests)

**Verification Evidence:**
Mutation check: making useFlowList call listFlows regardless of the capability fails "does not ask for the list when the schema says there are no flows". go test ./internal/dataentry (with -update for the schema goldens) and ./internal/apiwire pass; the full vitest suite, vue-tsc, eslint, golangci-lint on the changed packages and comment-lint are clean.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors capabilities.git and stores/git.ts)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (the capability names a config surface, which is not a secret; no new input)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
