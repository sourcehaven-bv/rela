---
id: IMPL-QGBSQHJ
type: implementation-checklist
title: 'Implementation: Bump vitest to 5.0.5 for GHSA-p458-7748-mcp3 (GH #1989)'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code; dependency bump only)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no new code; the existing suites run on the new vitest)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: none identified for a patch bump)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no code paths changed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests added)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: none identified for a patch bump)

**Verification Evidence:**
- osv-scanner 2.7.0 on `frontend/package-lock.json`: no issues found. On
origin/develop it reports GHSA-p458-7748-mcp3, GHSA-mrwp-f3q8-3g9p,
GHSA-3g3m-2fjr-6pw4 and GHSA-g3hv-75wg-m249.
- `npm ls`: follow-redirects 1.16.2, glob 14.0.3, minimatch 11.0.3 (one copy
each). glob 14 also brings path-scurry 3.0.0 and minimatch 11 adds
`@isaacs/nonrecursive-glob-match` 1.0.3.
- Lint reports the same 0 errors and 184 warnings as before the minimatch
override, so eslint's file matching is unchanged.
- `npm ls vitest @vitest/ui @vitest/browser`: 5.0.5 only. The lockfile also
moves dependencies that vitest 5.0.5 requires (chai 6.3.0, obug 3.0.0, tinybench
6.2.2, tinyrainbow 3.2.0, magic-string 1.4.5).
- Frontend: typecheck clean; lint 0 errors; `RELA_FULL_CORPUS=1 npm run
test:run` 359 files, 5480 tests pass; build succeeds.
- Component library: `vue-tsc -b` and `npm run check` pass; `npm test` (vitest
5.0.5, Playwright Chromium) 131 files, 640 tests pass. Work tree clean after the
build.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)~~ (N/A: version ranges only)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no code paths changed)
- [x] No debug code left behind
