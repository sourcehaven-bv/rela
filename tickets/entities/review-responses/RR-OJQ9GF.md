---
id: RR-OJQ9GF
type: review-response
title: Claim loop puts lease_id on argv
finding: CLAUDE.md examples pass the secret as an argument visible in ps and shell history (also raised by the security review)
severity: minor
resolution: 'Fixed in the review-fix commit (rela lease CLI: review fixes); error texts now come from lease.UserText shared with MCP.'
status: addressed
---
