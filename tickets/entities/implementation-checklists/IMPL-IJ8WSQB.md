---
id: IMPL-IJ8WSQB
type: implementation-checklist
title: 'Implementation: The tenant setting-up screen still ignores the app language'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (useShellAppConfig.test.ts)
- [x] Integration tests written (test full flow, not just units) (the tests write the boot meta tags and load locale.ts and the composable fresh, as a page load does, then drive a reactive store through load and reload)
- [x] Happy path implemented
- [x] Edge cases from planning handled (config without a language clears it; reload keeps the config; a shell without boot values keeps the store name)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error path; a failed config load leaves the shell values, which is the intended state)

## Test Quality

- [x] Using fixture builders or factories for test data (notLoaded() and addMeta helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the live pool screen needs a hub sign-in and a new org; verified instead that the live pool shell on 127.0.0.1:8210 carries lang="en", the app name and both metas, and that the test fails with the old App.vue watch)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: covered by the unit tests, see above)

**Verification Evidence:**
curl http://127.0.0.1:8210/ (pool-rela, develop f71d489e0) returns `<html lang="en">`, `<title>Sourcehaven Workspace</title>`, `rela-app-name` and `rela-language` metas, so the server side of BUG-21N3J4N works and no saas-dev setting is missing. Mutation checks: restoring the unguarded watch fails "keeps the shell language and name while the config has not loaded"; gating the name on loaded instead of the latch fails the reload test. vitest on the new test and src/i18n passes; vue-tsc and eslint are clean.

## Quality

- [x] Code follows project patterns (check similar code) (a composable like useDocumentTitle)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (no new input; the meta values are already validated and escaped by the server)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no error path)
- [x] No debug code left behind
