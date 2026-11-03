---
id: RR-RY8K0C
type: review-response
title: claim_entity drops non-string label or status
finding: claim_entity reads label and status with GetString which silently ignores non-string values while renew_lease rejects them; use optionalString and return its error
severity: significant
resolution: 'Fixed in the review-fix commit (Lease MCP tools: review fixes) with tests where behaviour changed.'
status: addressed
---
