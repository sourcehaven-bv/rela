---
id: RR-C6BZWT
type: review-response
title: OpenAPI 409 for delete omits channel block
finding: internal/openapi/paths.go:147 still says only has incoming relations.
severity: nit
resolution: The DELETE 409 description covers incoming relations, channel_blocks_delete and conflict; golden files regenerated.
status: addressed
---
