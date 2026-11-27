---
id: RR-AQ9OTPX
type: review-response
title: lowerRelationFilters trusts its caller's operator check
finding: 'It treats every operator except ne as eq and does not check arrayForm. Safe only because planListPushdown runs first. Fix: re-check with pushableRelationFilter inside the loop.'
severity: nit
resolution: relationFilterValues returns only eq and ne keys, so the operator is not checked twice.
status: addressed
---
