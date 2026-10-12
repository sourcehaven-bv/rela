---
id: RR-E139NM
type: review-response
title: AC3 not tested as written
finding: the test calls GetReadable with a 10 s wait and never checks release
severity: significant
resolution: 'Fixed in the review-fix commit (rela lease CLI: review fixes); error texts now come from lease.UserText shared with MCP.'
status: addressed
---
