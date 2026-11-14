---
id: RR-4LU5ZUD
type: review-response
title: Permanent create errors retried forever
finding: A create that fails for a reason retrying cannot fix (for example a schema collision) was treated like a transient failure, so every request and webhook retried it every 30 seconds without end and without telling an operator.
severity: significant
resolution: 'Fixed in 13cd61e18: the adapter marks such refusals with tenanthost.ErrNeedsOperator. The host holds them for 1 hour, acknowledges the webhook with 200 so the IdP stops retrying, and answers requests 503 provisioning_needs_operator. Pinned by TestProvision_NeedsOperator and TestProvisionAdapter.'
status: addressed
---
