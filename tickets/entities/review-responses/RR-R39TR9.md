---
id: RR-R39TR9
type: review-response
title: No OpenAPI goldens exist
finding: _comment_count appears only in internal/apiwire/v1/responses.go and frontend/src/types/entity.ts. The plan should drop the conditional "OpenAPI goldens, if ..." item from Files to modify.
severity: nit
resolution: The OpenAPI goldens item is dropped from Files to modify.
status: addressed
---
