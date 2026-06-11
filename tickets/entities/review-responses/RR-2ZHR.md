---
id: RR-2ZHR
type: review-response
title: E2E test verifies server state but not rendered UI state
finding: Un-skipped test polls server-side content via API. Does not assert SPA's rendered checkbox visibly updates. If loadView() silently failed or watcher misfired, test would still pass. This is the exact failure shape of the original bug (API works, UI doesn't).
severity: nit
resolution: 'Added a second `expect.poll` after the server-state assertion: `await expect.poll(() => entity.contentCheckboxIsChecked(0), { timeout: 2000 }).toBe(true)`. Now any future regression where the API call works but the SPA''s rendered state stays stale will be caught.'
status: addressed
---
