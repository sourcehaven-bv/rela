---
id: RR-TBG2YB
type: review-response
title: lease-id-stdin edge cases
finding: blocks on a terminal; whitespace input gives malformed instead of missing; over 1024 bytes silently truncated
severity: minor
resolution: 'Fixed in the review-fix commit (rela lease CLI: review fixes); error texts now come from lease.UserText shared with MCP.'
status: addressed
---
