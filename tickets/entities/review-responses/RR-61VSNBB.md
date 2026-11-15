---
id: RR-61VSNBB
type: review-response
title: RevalidateOrg ran under the webhook request context
finding: After an IdP suspension the host called Registry.RevalidateOrg with the webhook request context, so a client disconnect could cancel closing the open tenant.
severity: minor
resolution: 'Fixed in 13cd61e18: RevalidateOrg runs under context.WithoutCancel of the request context with a 10 second timeout. No dedicated test; covered by the existing status sync tests.'
status: addressed
---
