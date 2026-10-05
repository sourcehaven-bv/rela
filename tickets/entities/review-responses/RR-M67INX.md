---
id: RR-M67INX
type: review-response
title: Store faults logged as ACL failures on the document route
finding: '[security] On a non-default world getVisible returns store read errors, which writeGateError logs as an ACL check failure. No disclosure; misleading log.'
severity: nit
reason: Matches handleV1GetEntity, which routes the same getVisible errors through writeGateError. Changing the log category belongs in visibleReader for every caller, not in this route.
status: wont-fix
---
