---
id: RR-DD2Z0O
type: review-response
title: Conversion and copy mappers still answer 403 for a missing identity
finding: writeEntityMakingError and writeCopyError unwrap acl.ForbiddenError and write their own 403, so a no-identity deny on those routes stays 403.
severity: significant
resolution: Both call writeIdentityFaultDeny before their own 403. TestNoIdentity_SharedMappers covers writeConversionError and writeCopyError.
status: addressed
---
