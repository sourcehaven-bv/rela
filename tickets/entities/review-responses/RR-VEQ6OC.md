---
id: RR-VEQ6OC
type: review-response
title: Agent that loses its lease_id locks itself out
finding: after context loss the next claim is lease held and CLAUDE.md says pick another ticket; labels are free text so the agent cannot recognise its own lease
severity: significant
resolution: 'Fixed in the review-fix commit (rela lease CLI: review fixes); error texts now come from lease.UserText shared with MCP.'
status: addressed
---
