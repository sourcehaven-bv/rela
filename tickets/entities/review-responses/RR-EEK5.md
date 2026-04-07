---
id: RR-EEK5
type: review-response
title: Repo-root e2e/ suite still hardcodes /v2/ in base.page.ts, fixtures.ts, crud.spec.ts, data-entry.spec.ts
finding: 'The reviewer noted that there is a second Playwright suite at the repo-root e2e/ directory (distinct from frontend/e2e/) that still hardcodes /v2/ in its navigation helpers. Specifically: e2e/pages/base.page.ts:19 prepends /v2 to every path; e2e/tests/fixtures.ts:372 probes /v2/ for readiness; fixtures.ts:393 does page.goto(/v2/) on test start; crud.spec.ts:131 navigates to /v2/entity/task/...; data-entry.spec.ts:22 has URL regex /\/v2\/(dashboard)?/ and comment strings "Data Entry App v2". My original grep only scoped frontend/e2e/ and missed the repo-root e2e/ suite entirely. This would have been a silent breakage masked by the pre-existing waitForServer 403 bug.'
severity: significant
resolution: 'Edited all four files in e2e/: base.page.ts navigateTo now passes paths through untouched (just ensures leading /); fixtures.ts readiness probe now hits /; initial page.goto now uses /; crud.spec.ts navigates to /entity/task/... without prefix; data-entry.spec.ts URL regex updated and describe strings renamed from ''Data Entry App v2'' to ''Data Entry App''. Verified with `grep -rn /v2/ e2e/` returns no matches.'
status: addressed
---
