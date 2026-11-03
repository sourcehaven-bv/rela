---
id: RR-GMGO8A
type: review-response
title: Lease error texts duplicated between CLI and MCP
finding: five texts and the mapping chain are copied with no test pinning them; move the mapping into internal/lease
severity: significant
resolution: 'Fixed in the review-fix commit (rela lease CLI: review fixes); error texts now come from lease.UserText shared with MCP.'
status: addressed
---
