---
id: RR-T3PDHN
type: review-response
title: List-table renderer must live in internal/dataentry, not internal/transform
finding: The list-table renderer needs to reproduce relation-column cells = comma-separated neighbor TITLES. The wire response only carries neighbor IDs (entityserializer toV1); titles are resolved client-side from ?include. Reproducing cells server-side requires outgoing+incoming edges per row, each neighbor's full entity (for DisplayTitle), inverseRelationKey, AND the ACL neighbor-visibility gate (visibleRelationIDs/filterVisible, the RR-HJV8CP leak fix). All of this is unexported and ACL-coupled inside internal/dataentry. A plain []*entity.Entity slice is insufficient. Placing the list renderer in internal/transform would either leak hidden neighbor titles or require importing dataentry internals, which arch-lint forbids.
severity: significant
resolution: Split the Renderer implementations by layer. Built-in ENTITY renderer (single entity's own props/relations/body) lives in internal/transform (lower layer, no ACL machinery needed for its own resolved relations — or takes pre-resolved relation titles as input). The LIST-TABLE renderer lives in internal/dataentry where it can use entityReader, visibleReader, the batched neighbor gate, and inverseRelationKey. The transform Engine + Registry + Renderer interface stay in internal/transform; dataentry supplies a Renderer implementation. This keeps the ACL gate authoritative and satisfies arch-lint.
status: addressed
---
