---
id: RR-3VTHTVU
type: review-response
title: InferEntityType folding is untested
finding: The fold reaches InferEntityType through MatchesID, but no test pins it.
severity: minor
resolution: TestMetamodel_InferEntityType_CaseFold covers folded, exact-case, longest-prefix and tie-break answers, repeated to vary map order.
status: addressed
---
