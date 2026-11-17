---
id: RR-3KDMFJS
type: review-response
title: Position ignores the list condition
finding: 'GET /_position runs the list pipeline without the list''s condition: so a walk would count and link rows the list does not show.'
severity: significant
resolution: ScopeDescriptor gains list_id; resolveScope applies the named list's condition after the gated pipeline.
status: addressed
---
