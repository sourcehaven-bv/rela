---
id: RR-KC17CF
type: review-response
title: Docs describe an entities landing-page rule that has no effect
finding: firstNavTarget has no production caller; the SPA route / redirects to /dashboard. The new docs bullet and the firstNavTarget skip pinned behaviour nobody sees.
severity: minor
resolution: Reverted the firstNavTarget edit and its test and removed the docs bullet. The dead function and the stale landing-page sentence predate this ticket.
status: addressed
---
