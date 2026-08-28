---
id: RR-L4M1WK
type: review-response
title: Empty version history leaves bogus ?base/?target params in the URL unvalidated
finding: 'Both views gate the entire seed/recompute/publish block on `if (versions.length)` (HistoryView.vue:105, RelationHistoryView.vue:94). Opening /history/feature/FEAT-1?base=999&target=nonsense on an entity with no captured versions leaves those params in the address bar permanently — publishSelection() is never called. The refs correctly fall back to defaults so nothing breaks visually, but the promise that a bogus URL becomes an explicit correct one lapses exactly where the URL is most wrong. Bookmark trap: the user bookmarks it, the sweep later captures versions, and the stale params then get evaluated against a real list. docs/postgres-backend.md also makes the unconditional claim that a value naming no existing version falls back to the default — true of the view, not of the URL.'
severity: significant
resolution: 'Fixed in both views. The `if (versions.length)` gate now wraps only the recompute (there is nothing to diff with no versions); seeding and publishSelection() run unconditionally, so a stale or bogus param is always rewritten to the pair actually being shown. Comment at both call sites explains that the empty case is precisely when the URL is most likely wrong. Covered two ways: a unit test asserting publish() emits {base:''current'',target:''current''} when validVersions() is empty and the URL says ?base=999&target=nonsense, and an e2e test asserting an out-of-range ordinal is corrected to the default in the address bar. The docs sentence in docs/postgres-backend.md was also corrected — it now states that the address bar is rewritten to the pair being shown, so the corrected link is what gets copied.'
status: addressed
---

## Finding

`if (versions.value.length)` wraps seed + recompute + publish. With zero
versions, `publishSelection()` never runs, so malformed params survive in the
address bar indefinitely.

## Impact

Mostly cosmetic today (the refs fall back correctly, the view renders "No
versions recorded yet."), but it is a bookmark trap: version capture is
asynchronous, so an entity with no versions *now* may have them later, at which
point the stale params are evaluated for real.

The documentation in `docs/postgres-backend.md` states the fallback guarantee
unconditionally, which is accurate for the rendered view and inaccurate for the
URL.
