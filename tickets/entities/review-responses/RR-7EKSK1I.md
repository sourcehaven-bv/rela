---
id: RR-7EKSK1I
type: review-response
title: 'Design: CLI processes never revalidate'
finding: A long-running scheduler or stdio MCP session keeps serving a suspended tenant and blocks its erasure.
severity: significant
resolution: 'tenant.Watch re-resolves the tenant every 30s in rela --tenant processes (openProject); a refusal prints a message and exits, releasing the schema lock. Test: TestWatch, TestTenantCLI_StopsWhenSuspended.'
status: addressed
---
