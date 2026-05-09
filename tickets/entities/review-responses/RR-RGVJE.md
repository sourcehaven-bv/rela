---
id: RR-RGVJE
type: review-response
title: SSE event semantics on partial failure / no-op are not specified
finding: |-
    Plan says SSE events suppressed when no writes. Today entity SSE fires after successful entity update, then relation reconcile may fail (the relation 422 is returned but SSE has already gone out). Recommendation: specify 'on 422 from any phase, entity SSE does NOT fire'; per-relation events fire only on actual writes; entity event fires only when entity itself changed. Add AC for relations-only PATCH not emitting entity:updated.

    From design-review: F11.
severity: minor
resolution: 'Plan Layer 5 specifies SSE semantics: entity event only on actual entity change; per-relation events only on actual writes (after value-based no-op suppression); no events on 400/422; relations-only no-op PATCH emits zero events.'
status: addressed
---
