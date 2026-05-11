---
id: RR-U2S9
type: review-response
title: exportTheme revokes object URL before download initiates
finding: theme.ts:71-83 — `URL.revokeObjectURL(url)` runs synchronously in `finally` immediately after `a.click()`. Modern browsers snapshot, but Safari historically dropped downloads when the URL was revoked before the browser attached the download. Defer with `setTimeout(() => URL.revokeObjectURL(url), 0)` and drop the try/finally (click can't usefully throw).
severity: significant
resolution: exportTheme now defers `URL.revokeObjectURL(url)` via setTimeout(...,0). Removed the try/finally (click can't usefully throw). Updated theme.test.ts to await one macrotask before asserting the revoke.
status: addressed
---
