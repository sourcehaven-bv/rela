---
id: IMPL-69FYMO
type: implementation-checklist
title: 'Implementation: Predicate language: current_user with is_current_user/has_current_user sugar, pushed into next-action queries'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Unit: `internal/predicate/prefilter_test.go` (ConstEqualities per shape,
References/Functions, one expression per IR node type),
`internal/predicatefns/currentuser_test.go` + `evaluator_currentuser_test.go`
(bind fail-closed, MatchesAs binds only when required, prefilter spec drift
guard incl. RequiresCurrentUser, empty identity never matches),
`internal/queryplan/conditionprefilter_test.go` + `queryplan_test.go` (lowering,
index derivation, agreement, static specs, Load refuses a broken condition),
`internal/conditionlint/nextaction_test.go` (user profile, identity contract,
free-text refusal, Types), `internal/nextaction/condition_test.go`
(ErrIdentityRequired skips only that source; other errors propagate),
`internal/appbuild/nextaction_matchers_test.go` (scope binder, agreement,
conflict), `internal/affordances/currentuser_sugar_test.go` (sugar; unknown
placeholder via `everyone`).

Integration: `internal/dataentry/nextaction_condition_test.go` — pre-filter
reaches the store while the Go pass stays authoritative; real
`appbuild.NextActionMatchers` behind the HTTP handler for all three spellings
across principals; unidentified caller gets the identity-free source and null
when only per-user sources exist; identity conflict →
`next_action_identity_conflict` with no identity echoed; hidden property makes
the condition false end to end.

Errors: an unidentified caller's per-user source is SKIPPED with a WARN naming
the source (never a placeholder match, never a failure of other sources); a
stamp/principal conflict is refused whole with a named code; a non-compiling or
free-text condition is a load error.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Fixtures: `seedAssignedTicket`, `withTicketAssignment`, `recordingMatcher`,
`matcherFuncFor`, `scoped`, `prefilterEnv`/`fullSpec`,
`matcherMeta`/`matcherCfg`, `matchIDs`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Scratch project (`/tmp/rela-cu-verify`): `task` with `assignee` (string) and
`watchers` (string list); four tasks (assigned to alice, to bob, to carol with
watchers [dave, alice], unassigned); one source `query: "type:task
prop:status=open"`, `condition: "is_current_user(entity.assignee) or
has_current_user(entity.watchers)"`. `rela-server -principal-header
X-Forwarded-User`, `GET /api/v1/_next_action`, repeated on the final tree (after
the code-review fixes):

| principal | result |
|---|---|
| alice | `T-watched` (eligible for both her own and the watched task; stable-random picked the latter) |
| bob | `T-bob` |
| dave | `T-watched` (watcher) |
| erin | `{"suggestion": null}` |
| no header (placeholder principal) | HTTP 200 `{"suggestion": null}`; WARN `nextaction: source skipped, its condition needs an identified principal source=assigned`; no identity or entity data in body or log |

Gates on the final tree: full `./internal/... ./cmd/...` suite, golangci-lint,
arch-lint, comment-lint, plimsoll, coverage floors (79.3%) — all pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns: consumer-side interfaces (`ConditionPrefilterer`,
`NextActionRequestScope`, `PrincipalResolver`), optional capability by type
assertion, belt-and-braces pushdown identical to `PushdownPrefilters`, shared
eligibility core for pushdown and index inference, `principal.Unknown` replaces
three bare literals, sugar bindings built once per binding context. Security:
design review RR-2ITZ84/RR-LD2B23 addressed; code review's security pass found
no issues.
