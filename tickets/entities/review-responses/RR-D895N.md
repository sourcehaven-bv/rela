---
id: RR-D895N
type: review-response
title: meta_unset of a key not present on the edge is unspecified
finding: |-
    Plan covers schema-known but doesn't say what happens for declared-but-not-currently-set keys. Go delete() is a no-op, but combined with F3's shape-based suppression: re-PATCHing `meta_unset: ["X"]` for absent X always trips the 'has MetaUnset → upsert' branch and writes every time. Recommendation: 'meta_unset of declared-but-absent key is no-op'. Combine with F3 by computing post-merge map and comparing to current for true value-based suppression.

    From design-review: F9.
severity: significant
reason: Dissolves under DEC-HWZHA. meta_unset of a declared-but-absent key is now an explicit no-op (Go delete() of a missing key is silently fine). The combinatorial worry with no-op suppression (always tripping the upsert branch) is addressed by RR-M3LWM's value-based no-op suppression, where the post-merge map is compared against the current map; if they're equal, no write.
status: wont-fix
---
