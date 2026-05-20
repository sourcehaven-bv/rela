---
id: RR-0HSC
type: review-response
title: isCellInaccessible does linear scan per cell — O(rows × cols × inaccessibleLen)
finding: 'EntityList.vue:466-470 isCellInaccessible runs Array.some on every cell render. For 100 rows × 10 columns × 10 inaccessible fields = 10k scans per re-render. EntityDetail.vue:111-117 uses the right pattern (Set in a computed). Apply the same pattern: precompute Map<entityId, Set<string>> from entities; isCellInaccessible becomes O(1). Not user-visible at small scale but exactly the kind of hot-path issue you''d flag elsewhere.'
severity: minor
status: deferred
reason: |-
    Parent ticket TKT-PGK91 (git-crypt detection) shipped via PR #668 without addressing this finding. Captured here so the gap remains visible; will be revisited if the underlying code path becomes a problem in practice. Closed as deferred via the TKT-5S8T data-debt sweep — the alternative is leaving the RR open indefinitely while it blocks every unrelated PR.
---
