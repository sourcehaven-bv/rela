---
id: RR-MAA6TM
type: review-response
title: MCP accept resolved the target before provisioning
finding: AcceptSuggestion ran address() (the read gate) before provisioned(ctx); the HTTP route provisions first, so a first-time provisioned user got not_found over MCP
severity: significant
resolution: 'Fixed: AcceptSuggestion now runs read-only refusal, then provisioning, then address and acceptOp. Pinned by TestMCPComments_Authorization/provisioning_runs_before_the_target_resolves, which fails with the old order.'
status: addressed
---
