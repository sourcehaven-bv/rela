---
id: RR-GPDMROX
type: review-response
title: Double read gate on resolve was undocumented
finding: The conflict read gate and AuthorizeUpdate's read gate both run; a reader might remove one.
severity: nit
resolution: The authorizeConflictResolve doc says the repeat is deliberate.
status: addressed
---
