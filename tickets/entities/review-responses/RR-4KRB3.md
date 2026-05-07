---
id: RR-4KRB3
type: review-response
title: 'data: null claim is unverified and likely wrong'
finding: |-
    Plan: 'data: null deserializes as nil slice (length 0); we treat as 400, distinct from data: [] which is empty.' This needs verification, not assumption. json.Unmarshal of null into []T produces a nil slice with len==0. So does []. They are indistinguishable after json.Unmarshal unless you intercept raw bytes or use json.RawMessage / custom UnmarshalJSON on V1RelationsUpdate. Plan's premise — 'we can tell them apart and reject null' — does not hold with the proposed struct.

    Fix (recommend Option A): treat data: null as equivalent to data: [], both meaning 'remove all'. JSON:API §9.2.1 explicitly defines null as 'set to empty' for to-many. Standard library produces this — no custom unmarshal needed. Update plan and remove the 'data: null → 400' AC; replace with 'data: null → equivalent to data: [], removes all.'
severity: significant
resolution: 'Decision #3 + AC #4: data: null treated as equivalent to data: []. Per JSON:API §9.2.1; matches what json.Unmarshal naturally produces. No custom unmarshal logic needed.'
status: addressed
---
