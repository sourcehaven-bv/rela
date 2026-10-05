---
id: RR-MWJUT4
type: review-response
title: 'PR 8 security: AllFaces pushdown delta for scoped principals'
finding: Scoped principals now get granted face rows through AllFaces scans while the bare-id Resolver and GET still gate on the default-world row; untested.
severity: minor
resolution: Added TestListPushdown_AllFacesScopedPrincipalGetsGrantedFaceRowsOnly with a real acl.Declarative; delta recorded in the PR body as consistent with the grant until TKT-7IZHP0.
status: addressed
---
