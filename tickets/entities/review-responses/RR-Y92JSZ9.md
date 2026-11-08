---
id: RR-Y92JSZ9
type: review-response
title: Schema pattern accepts public and information_schema
finding: A tenant on public would be served from the shared fallback schema, and erasing it would drop pg_trgm for every tenant.
severity: significant
resolution: checkSchemaName refuses public, information_schema and pg_ names in validate and before DSN derivation. Pinned by TestErase_RefusesSharedSchemas.
status: addressed
---
