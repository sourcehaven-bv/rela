---
id: IMPL-NM5UK
type: implementation-checklist
title: 'Implementation: Extract stubEntityManager to shared test helper package'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: PanicOnUse contract is "every method panics" — exercised by absence of panics in existing test suites)
- [x] ~~Integration tests written~~ (N/A: helper has no behaviour beyond panicking; exercised via existing dataentry/script tests)
- [x] Happy path implemented (PanicOnUse satisfies entitymanager.EntityManager via compile-time assertion)
- [x] Edge cases from planning handled (interface evolution caught by `var _ entitymanager.EntityManager = PanicOnUse{}`)
- [x] ~~Error handling in place~~ (N/A: panicking is the contract)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no test data introduced; only stub replacement)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no new assertions)
- [x] ~~Only specifying values that matter for the test~~ (N/A: mechanical refactor)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolation)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no new comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (covered by compile-time interface assertion + existing test runs)

**Verification Evidence:**
- `go build ./...` clean.
- `go test -race ./...` — entire repo passes; `internal/dataentry` and `internal/script` (the two consumers) pass without changes to test logic.
- `just lint` — 0 issues.
- `just arch-lint` — only pre-existing `.ignored/` notices on develop; new `entitymanagertest` is properly excluded.

## Quality

- [x] Code follows project patterns (mirrors `internal/store/storetest` layout and arch-lint exclusion)
- [x] No security issues introduced (test-only helper)
- [x] No silent failures (panic on use is the explicit contract — opposite of silent)
- [x] No debug code left behind
