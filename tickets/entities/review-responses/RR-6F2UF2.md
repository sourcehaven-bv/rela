---
id: RR-6F2UF2
type: review-response
title: filter.SortMulti's descending path is a broken comparator
finding: 'Design review C1. Every sort helper in internal/filter/sort.go uses `if descending { return !less }` (:56-59, :239-241, :299-302). For two equal keys less(i,j) and less(j,i) are both false, so !less reports BOTH orderings - not a valid strict weak ordering. Independently verified: 13 identical keys sorted descending return MLKJIHGFEDCBA, i.e. reversed, not stable. Worse, because SortMulti applies keys in reverse-priority order relying on stability (:193-196), a multi-key sort corrupts the secondary key: primary status desc + secondary due asc returned due fully DESCENDING (r11:2026-12-01, r10:2026-11-01, ...). The plan''s proposed id tiebreak does not fix this and makes it worse: a tiebreak inside `less` gets inverted by !less, so ties would break by id DESC on descending sorts while SQL and applyV1Sorting both emit `e.id ASC` unconditionally (api_v1.go:2249, naive.go:124, pgstore/graphquery.go:541). Fix requires three-way (int) comparators across compareEnums/compareStrings/compareDates/compareIntegers/compareBooleans/comparePropValues, inverting only the key comparison and applying the id tiebreak outside the inversion. This invalidates the plan''s claim that step 1 writes no new comparison code and is small.'
severity: critical
status: open
---
