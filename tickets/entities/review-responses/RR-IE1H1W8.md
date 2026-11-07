---
id: RR-IE1H1W8
type: review-response
title: 'Security: identity mismatch answers 500'
finding: acl.ErrIdentityMismatch falls through to a generic 500 rather than an auth status.
severity: minor
reason: 'Project convention (writeNoIdentity, writeIdentityFaultDeny): a mismatched bound Request is a server fault and stays 500; the detail is generic and the ids go to the log.'
status: wont-fix
---
