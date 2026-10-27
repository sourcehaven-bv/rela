---
id: RR-WDXGFG
type: review-response
title: Insert values into response objects only
finding: Writing per-principal values into entity.Properties feeds ETags, exports, versions and may mutate a shared Redact result.
severity: significant
resolution: Values are a side map merged only into the v1.Entity wire object and the MCP DTO, after the ETag is computed. entity.Entity is never mutated.
status: addressed
---
