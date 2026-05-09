---
id: RR-PZY1W
type: review-response
title: Validation order between entity validation and relation validation is unspecified
finding: |-
    Plan doesn't say whether modern reconciler validation runs before or after entityManager.UpdateEntity. Affects which 422 the user sees first, AC17 atomicity, future parallelization. Same axis as F2 but specifically about validation. Recommendation: specify 'modern reconciler validation runs FIRST, before entity update; on validation failure return 422 with offending pointer, no writes occur; entity validation errors come from UpdateEntity itself.'

    From design-review: F5.
severity: significant
resolution: Plan Layer 3 specifies relation-validation runs FIRST, before entity update. Multiple 4xx errors are not combined; user sees the first one. Future refinement out of scope.
status: addressed
---
