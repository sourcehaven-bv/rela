---
id: RR-ARWVX6I
type: review-response
title: Lazy provisioning answered 202 to writes
finding: A write for a tenant being created got 202 with a provisioning status. The SPA treats 202 as success, so the user saw a write succeed that was never applied.
severity: significant
resolution: 'Fixed in 13cd61e18: only GET and HEAD get 202 provisioning; any other method gets 503 tenant_provisioning with Retry-After. Pinned by TestProvision_WriteDuringProvisioningIsRefused.'
status: addressed
---
