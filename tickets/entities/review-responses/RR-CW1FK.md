---
id: RR-CW1FK
type: review-response
title: AC17 contradicts atomicity-out-of-scope statement
finding: |-
    AC17 says 'PATCH with valid properties + invalid relation entry → entity NOT updated' but current dispatch order is entity-update first (api_v1.go:552) then reconcile (api_v1.go:558). Entity is on disk before relation validation runs. AC17 is unachievable without reordering. Plan says 'validation runs before any writes' but only inside the modern reconciler, not across phases. Recommendation: pick option A (validate-first reorder) or drop AC17.

    From design-review: F2.
severity: critical
resolution: 'Plan Layer 3 reorders the handler to validate-first: Phase A runs the relation validation pass (no writes), Phase B runs entity UpdateEntity, Phase C runs relation writes. A relation 400/422 returns before UpdateEntity, making AC17 achievable. Mid-write-loop store failures still leave a partial state — documented as a known atomicity limitation.'
status: addressed
---
