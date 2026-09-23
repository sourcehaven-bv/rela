---
id: RR-UQQ21I
type: review-response
title: X-Total-Count unreadable via api.get; Plural duplicates getPlural
finding: api.get returns only the body (api/client.ts). The wire Plural duplicates getPlural() (api/entities.ts:37).
severity: minor
resolution: Read ListResponse.meta.total. Drop Plural from the wire.
status: addressed
---
