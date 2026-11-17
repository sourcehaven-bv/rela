---
id: RR-A1LYBQ2
type: review-response
title: SPA did not poll on 503 or reload on 401/403 while provisioning
finding: The SPA did not poll on the 503 provisioning answers, and did not reload on 401 or 403 while polling the tenant status.
severity: minor
resolution: The SPA poller (frontend/src/api/tenantProvisioning.ts) now polls on 503 too and reloads the page on 401 or 403 (tenantProvisioning.test.ts). Commit f2f2dac83.
status: addressed
---
