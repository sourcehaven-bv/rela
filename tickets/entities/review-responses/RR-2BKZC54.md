---
id: RR-2BKZC54
type: review-response
title: Pushdown parity tested only under allow-all
finding: 'assertRelationIDPushdown uses Read: [*], so the gated hop never adds an endpoint predicate. No test compares pushdown and Go path for a row-gated target type (when: or role_relations), the faced-target decline (RR-DQFDRXO) or the named-face ErrTraversalUnsupported decline, on memory and postgres.'
severity: significant
resolution: Added a row-gated parity test (local role through role_relations) on memory and postgres, and TestLowerRelationFilters_Declines for a faced row-gated target and an unsupported traversal.
status: addressed
---
