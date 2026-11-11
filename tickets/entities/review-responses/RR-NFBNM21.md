---
id: RR-NFBNM21
type: review-response
title: 'Code review: shutdown does not wait for revalidation'
finding: th.close could run while a Revalidate pass still used the registry and resolver.
severity: minor
resolution: runTenantMode waits for the revalidate goroutine before th.close.
status: addressed
---
