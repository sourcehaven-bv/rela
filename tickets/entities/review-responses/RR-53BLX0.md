---
id: RR-53BLX0
type: review-response
title: '[security] Document export timing tells absent ids from hidden ones'
finding: export_document resolved the entry with untypedRef. That returns right after the stored-type read when the id is absent but runs the gate when the id exists. The response time told a hidden id from an absent one (RR-NGMI). Raised by both reviewers.
severity: significant
resolution: An absent id now runs the same gated read against the document's entity_type. A miss therefore costs at least what a denial costs. The 404 and the type-mismatch 400 are unchanged.
status: addressed
---
