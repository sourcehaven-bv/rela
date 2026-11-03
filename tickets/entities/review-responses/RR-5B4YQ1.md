---
id: RR-5B4YQ1
type: review-response
title: 'Review nits deferred: test hook export, cross-tab sync, translated group id'
finding: resetLanguageState is exported from a production module; no storage or languagechange listener; the Apps group id is built from its translated label.
severity: nit
reason: The test hook is small and named as such. Cross-tab sync and stable group ids belong with the full-screen translation in TKT-5GX50Y; the Apps label is identical in both languages today.
status: deferred
---
