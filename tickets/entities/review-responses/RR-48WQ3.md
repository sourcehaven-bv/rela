---
id: RR-48WQ3
type: review-response
title: Legacy IDs-only shape on content-bearing/required-meta relation types unspecified
finding: |-
    Legacy reconciler creates with empty RelationOptions{}. Required-meta and required-content rules are advisory at write-time today, so legacy succeeds with sparse edges. Modern shape enforces closed-schema meta validation; users upgrading callers see new 422s where old code created sparse edges. Recommendation: document the asymmetry in api-reference.md as backwards-compat caveat.

    From design-review: F10.
severity: significant
reason: 'Dissolves under DEC-HWZHA. Both legacy and modern shapes now WARN (not 422) on missing required meta or content. The asymmetry the reviewer flagged is gone: both shapes write the edge and surface the warning. Backwards-compat caveat reduces to ''modern shape includes warnings array; legacy shape doesn''t surface warnings inline (read analyze tools).'' That''s a minor doc note, not a blocking issue.'
status: wont-fix
---
