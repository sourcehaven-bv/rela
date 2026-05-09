---
id: RR-TK2QF
type: review-response
title: Concurrent automation between entity update and relation writes can stale the diff
finding: |-
    Sequence (after F2 reorder): relation-validate → entity update (fires automations, possibly creating relations) → relation writes. If an automation creates a relation conflicting with the desired set, reconciler's diff is stale and may delete/recreate. Same hazard exists today; not worse, but plan doesn't acknowledge. Recommendation: document in risk section as known interaction. Or: re-fetch current outgoing relations after entity update before write phase.

    From design-review: F15.
severity: minor
resolution: 'Plan Layer 6 acknowledges the automation/diff staleness window: with validate → entity update → relation writes ordering, automations during entity update may create relations before the relation-write phase runs; the relation-write phase doesn''t re-validate against the post-automation graph. Same hazard as today''s legacy reconciler. Documented in api-reference.md as known interaction.'
status: addressed
---
