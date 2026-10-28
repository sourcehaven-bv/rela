---
id: RR-HM3ZCX
type: review-response
title: No guard stops a new mapper from writing its own 403
finding: Each handler can map ForbiddenError its own way, which is how the bug arose.
severity: significant
resolution: TestForbiddenMappersHandleIdentityFaults fails when a non-test dataentry file unwraps acl.ForbiddenError without writeIdentityFaultDeny, writeForbiddenIfACLDenied or failACLDenied. settings_handlers.go and caldav_write.go are exempt with reasons.
status: addressed
---
