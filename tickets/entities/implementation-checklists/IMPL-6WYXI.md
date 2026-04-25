---
id: IMPL-6WYXI
type: implementation-checklist
title: 'Implementation: Lint rule to flag pure-API test patterns in e2e specs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: a single eslint `no-restricted-syntax` selector. There is no separate unit. Verified by running eslint against canary specs that exercise positive and negative cases — see Manual Verification.)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: same reason. The full flow IS `npm run lint` against representative specs.)
- [x] Happy path implemented (rule fires on `api.rawRequest` inside `test()` bodies)
- [x] Edge cases from planning handled (`test.only`/`skip`/`fixme` matched; setup hooks exempt; page objects exempt by file scope)
- [x] Error handling in place (eslint reports a clear, actionable message)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no production test data is touched. Canary specs were throwaway and deleted.)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Procedure: each scenario was verified locally by writing a throwaway
`tests/_canary_positive.spec.ts`, running `npm run lint` from `e2e/`, and then
deleting the canary.

| AC | Scenario | Result |
|---|---|---|
| AC1 | `api.rawRequest('GET', \`features/${SEED.features.exportData}/relations/blocks?direction=incoming\`)`inside`test('canary fires the rule', ...)`| eslint emits 1 error:`Do not use api.rawRequest inside a test() body — add a Go integration test instead`(file`_canary_positive.spec.ts:5:22`, rule `no-restricted-syntax`). Pass. |
| AC2 | Existing spec tree, no source changes | `npm run lint`exits 0. Pass. |
| AC3 |`api.rawRequest('GET', 'features')`inside`test.beforeEach(...)`, only `appPage`in test bodies |`npm run lint`exits 0 with no errors. Pass. |
| AC4 |`e2e/tests/AGENTS.md` updated | Added a new section "API-only assertions belong in Go" before "Security canary lives in Go" that documents the rule, scope (`test`/`test.only`/`test.skip`/`test.fixme`), the hook exemption, and the rationale. Pass. |
| AC5 | User's exact illustrative example | Same as AC1 — fires on the literal example. Pass. |
| Bonus | `test.skip(...)`body containing`api.rawRequest` | eslint emits the same error. Pass. |

## Quality

- [x] Code follows project patterns (mirrors the existing `request.fetch` ban entry in the same file)
- [x] No security issues introduced (static lint rule, no runtime surface)
- [x] No silent failures (eslint surfaces the violation with a clear, actionable message)
- [x] No debug code left behind (canary specs deleted; `ls tests/_canary*` returns no matches)
