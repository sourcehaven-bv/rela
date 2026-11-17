---
id: RR-C3TADRN
type: review-response
title: organization.created webhook acknowledged before the tenant existed
finding: The organization.created webhook returned 200 before the tenant existed, so a create that failed afterwards was never retried by the IdP.
severity: critical
resolution: The webhook now waits up to 10s for the create (webhookProvisionWait). It returns 200 only once the tenant exists, and 503 with Retry-After otherwise, so the IdP delivers the event again (TestHost_WebhookProvision). Commit f2f2dac83.
status: addressed
---
