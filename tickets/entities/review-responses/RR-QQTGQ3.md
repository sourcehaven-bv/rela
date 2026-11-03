---
id: RR-QQTGQ3
type: review-response
title: cli-reference overstates release and omits errors
finding: release does not always succeed; store full / not enabled / forbidden / ambiguous missing; every failure exits 1
severity: minor
resolution: 'Fixed in the review-fix commit (rela lease CLI: review fixes); error texts now come from lease.UserText shared with MCP.'
status: addressed
---
