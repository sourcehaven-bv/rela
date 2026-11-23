---
id: RR-2ZOVQDO
type: review-response
title: Unreachable PropCond fallback drops the whole filter
finding: 'Code review: the fallback could fall through to ConditionSuperset instead of reading the type unfiltered.'
severity: nit
reason: Unreachable while related() is refused at compile; the fallback is correct, only slower.
status: wont-fix
---
