---
id: RR-3COZCN
type: review-response
title: Doc ACL assertions describe rename on the default face only
finding: '[security] internal/docs/assert_acl.go evaluates allows{op=rename} with a faceless subject, so a manual could claim a type-wide grant allows renaming a faced type.'
severity: nit
reason: 'Out of scope for this bug: a documentation-assertion surface, not enforcement. It affects delete equally and belongs with the DEC-NPZICR staging that removes the zero-face API.'
status: deferred
---
