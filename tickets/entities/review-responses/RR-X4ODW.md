---
id: RR-X4ODW
type: review-response
title: Self-loops skipped only for symmetric, not for inverse
finding: |-
    api_v1.go:856-874: symmetric branch has `if rel.From == rel.To { continue }` but the inverse branch doesn't. PATCH that adds A.T → A on a relation with Inverse: {ID: 'T-inv'} stages a back-edge (A, T-inv, A). The comment on lines 845-847 claims self-loops are handled — only for symmetric.

    Fix: add `if rel.From == rel.To { continue }` in the inverse branch too, OR document why inverse self-loops are intended. Add tests for both cases.
severity: significant
resolution: 'propagateRelations: inverse branch now also has `if rel.From == rel.To { continue }`. Tests TestV1Patch_SymmetricSelfLoop and TestV1Patch_InverseSelfLoopSkipped verify both branches skip self-loops correctly.'
status: addressed
---
