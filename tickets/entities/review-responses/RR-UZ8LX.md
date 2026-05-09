---
id: RR-UZ8LX
type: review-response
title: 'data: null semantics not specified'
finding: |-
    AC5 covers data:[]; AC8 covers no data key (400); plan implies data:null is distinguishable from data:[] but doesn't say what it means. Recommendation: pick one — reject data:null as 400 with data_required (same as missing key) OR treat as equivalent to data:[]. Document.

    From design-review: F14.
severity: minor
resolution: 'Plan Layer 1 specifies `data: null` returns 400 `data_required` (same as missing). `null` as relation value returns 400 `relation_value_null`. AC20c covers `data: null`. AC9 (unknown relation type) is hard 422 because there''s no defined storage location.'
status: addressed
---
