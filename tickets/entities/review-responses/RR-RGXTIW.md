---
id: RR-RGXTIW
type: review-response
title: Deny mapper checks the whole error chain instead of Decision.Cause
finding: errors.Is on the whole chain would turn a rule deny joined with an unrelated ErrNoIdentity into a 401.
severity: nit
resolution: writeIdentityFaultDeny and failACLDenied judge fe.Decision.Cause only. The rule deny joined case pins it.
status: addressed
---
