---
id: RR-XWOQZH
type: review-response
title: Results from the previous scope stayed on screen under the new chip
finding: '`selectType(''ticket'')` rendered the chip immediately but kept the unscoped rows for the debounce plus round-trip, so the menu displayed ''ticket'' above a list containing research entities and decisions. The chip is a claim about provenance; a row shown under it that came from a different query misattributes where the data came from. Not an ACL bypass (every row was server-gated) but a UI that contradicts itself, and an Enter in that window inserted a row the chip said was out of scope.'
severity: critical
resolution: '`applyScopeChange()` clears `state.items` on both scope set and scope clear, mirroring the existing below-MIN_SEARCH_LEN branch of `scheduleSearch`, which already drops a result set that no longer answers the question on screen (and documents that reasoning). Pinned by ''drops results from the previous scope so the chip cannot lie'' and ''drops results again when the scope is cleared''.'
status: addressed
---
