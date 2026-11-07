---
id: RR-CX3JB9Q
type: review-response
title: 'Code: sqlite reports a committed write as failed'
finding: On the root store, UpdateEntity and UpdateRelation refill the element after the write committed, and SwapRelationEndpoints fills after its UPDATE. An error there was returned, so the caller saw a failed write that had happened.
severity: significant
resolution: The refills and the swap fill now log a warning (warnElementLeftNull) instead of returning the error. The trigger guarantees the element is NULL, never stale, and the sweep heal fills it.
status: addressed
---
