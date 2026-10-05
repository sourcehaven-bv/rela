---
id: RR-MJ814N
type: review-response
title: acl.PerEntityVerdicts unused in production
finding: readableFacesMany builds the FaceVerdicts literal directly.
severity: nit
reason: Test gates build verdicts from outside the acl package (visibilitytest.IDVerdicts and several fakes), which cannot reach the unexported fields.
status: wont-fix
---
