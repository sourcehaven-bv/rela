---
id: RR-VNZV71
type: review-response
title: loadRowContent doc claimed a redaction re-projection it does not implement
finding: The comment said the whole entity is re-projected onto the row's redacted property set; the code copies Content only. Behaviourally fine (Redact is property-only) but a false security claim.
severity: significant
resolution: 'Comment rewritten to state what is true: only the body is copied, redaction is property-only, the redacted property map is left exactly as gated.'
status: addressed
---
