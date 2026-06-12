---
id: RR-Q1CFXV
type: review-response
title: 'Minor cleanups: optimistic-remove early-return, plural test-reset, loading comment'
finding: (#5) beginOptimisticRemove cancelled the query and wrote a copy even when the row wasn't on the visible page; (#6) _setEntityPluralForTest had no reset helper, risking cross-test registry pollution; (#4) the loading comment implied placeholderData keeps rows visible during pagination, which it doesn't (param change = new key = pending).
severity: minor
resolution: beginOptimisticRemove now early-returns (undefined optimistic) when no row matches — no cancel/churn, settle's prefix-invalidate still covers other pages. Added _resetEntityPluralsForTest(). Corrected the loading comment to distinguish same-key SSE refetch (no spinner) from param-change (spinner, as before).
status: addressed
---
