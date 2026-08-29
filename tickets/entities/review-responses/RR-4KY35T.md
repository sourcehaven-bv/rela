---
id: RR-4KY35T
type: review-response
title: Comma-bearing enum value matched wrong rows; ne failed to exclude
finding: 'The in/ne wire format joined repeated params with commas then split them back, so a value legitimately containing a comma ("Legal, Risk & Compliance") was torn into two members. Confirmed empirically: filter[tags][in] returned the WRONG entity, and filter[tags][ne] returned the row it was asked to exclude — the exact wrong-answer class BUG-AMK38R exists to fix, reintroduced through a different door. Routing the frontend multi-select through `in` is what made it reachable.'
severity: critical
resolution: 'Two changes. Backend: filterSetValues now keys off the explicit `[]` array suffix rather than the param count — with the suffix each repeated param is one member taken verbatim, without it a param splits on commas (the documented form). The count is not a usable signal because a one-element array is indistinguishable from a comma list by count, which was the first fix attempt and still tore the value. Frontend: a SINGLE selection now stays on `=`, which compares the value whole and is always comma-safe; only multi-selection uses `in`. Pinned by two edge-case tests (in matches the comma-bearing row, ne excludes it).'
status: addressed
---
