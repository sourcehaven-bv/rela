---
id: RR-IOR3UK
type: review-response
title: Claim failure logs drop the entity id
finding: leaseFailed logs only user and tool on claim backend errors; the entity id is not secret and should be logged
severity: minor
resolution: 'Fixed in the review-fix commit (Lease MCP tools: review fixes) with tests where behaviour changed.'
status: addressed
---
