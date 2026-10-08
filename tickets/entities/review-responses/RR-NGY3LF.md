---
id: RR-NGY3LF
type: review-response
title: Rename count ignored the tail face and regressed MCP
finding: '[security] readableRelationCount counted edges tailed at the renamed entity''s own hidden face, so rename_entity reported more than the gated store count MCP used before.'
severity: significant
resolution: readableRelationCount uses edgeVisibility. Pinned by TestRename_RelationsUpdatedSkipsEdgesOfHiddenFaces.
status: addressed
---
