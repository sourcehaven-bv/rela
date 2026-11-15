---
id: RR-GIQ04LL
type: review-response
title: Tenant stuck in provisioning answered 202 forever with no log
finding: A tenant left in status provisioning (for example after a crashed create in webhook mode) answered 202 to every request indefinitely and nothing was logged, so an operator had no signal.
severity: minor
resolution: 'Fixed in 13cd61e18: the host logs a rate-limited warning for a tenant that stays in provisioning. Pinned by TestProvision_StuckProvisioningIsLogged.'
status: addressed
---
