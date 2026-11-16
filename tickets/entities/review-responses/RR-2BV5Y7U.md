---
id: RR-2BV5Y7U
type: review-response
title: SetAttachFiler panics on nil Services
finding: SetAttachFiler(nil, f) dereferences s.
severity: nit
reason: Same shape as SetAttachRenderer; callers always pass assembled Services.
status: wont-fix
---
