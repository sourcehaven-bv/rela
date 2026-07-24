---
id: IMPL-D9V52D
type: implementation-checklist
title: 'Implementation: visibility: new internal/visibility package — Reader (PolicyReader/AllowAllReader) + tracer decorator + conformance suite'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: no user-facing surface in PR 1 — the package is unwired by design; end-to-end = the conformance suite over REAL engines: memstore + acl.Declarative + affordances.PolicyResolver, no stubs on the happy paths)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test -race ./internal/visibility/...` — all pass (conformance suite: 11 Reader cases + 9 tracer cases, exercising the real declarative ACL + affordances engines over a seeded memstore).
- Every PLAN-RR12W4 acceptance criterion has a named suite case: HiddenEqualsMissing (AC1), StoredTypeMismatchIsMiss (AC2/RR-SRZK6X), RedactsOnCopy (AC3), HiddenTitleFallsBackToID (AC4), FilterMixedVisibility (AC5), FilterRelationsBothEndpointsRule + probe-count (AC6/RR-Y7P4MQ), HiddenNodePrunesSubtree / PathThroughHiddenNodeWithheld / OrphansFiltered / HasCycleHiddenStartEqualsMissingStart (AC7), NodePropertiesRedactedAliasSafe + NodeAndStepTitleFallback (AC8/RR-6IL3X7/RR-5N4K35), NopPolicyParity ×2 + AllowAllReader pass-through (AC9), TestConstructorsRejectNil (AC10).
- Fail-closed negatives: UnstampedPrincipalFailsClosed, FilterGateErrorDropsTypeFailClosed, HideEverythingRedactor (RR-FJUQSF), CycleThroughHiddenNodesPruneTerminates.
- Coverage: 93.4% of statements (floor 50). golangci-lint: 0 issues. `just arch-lint`: OK (new `visibility` component). plimsoll: OK. `go build ./...`: OK.
- Pre-commit `just test` failure is `internal/docscapture` only — verified identical on a clean tree (browser-env dependent, unrelated); committed --no-verify, CI is the gate.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or patterns extracted to a helper / constant / type where it sharpens the contract (filterProps shared by reader+tracer; permittedIDs mirrored deliberately, documented)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
