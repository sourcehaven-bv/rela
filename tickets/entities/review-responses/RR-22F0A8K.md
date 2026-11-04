---
id: RR-22F0A8K
type: review-response
title: File list errors
finding: internal/openapi/schemas.go does not describe view sections; the goldens are testdata/golden/{readonly,decl-header}/view*.golden; the entry flag also appears on side-panel and custom view responses.
severity: nit
resolution: Plan file list corrected; openapi dropped; goldens named; the extra places noted for the field's doc comment.
status: addressed
---
