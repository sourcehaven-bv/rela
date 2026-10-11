---
id: RR-4N0I3K
type: review-response
title: list reads unprovisioned while run provisions
finding: list_actions uses the unprovisioned ctx and run_action the provisioned one, so the two can disagree for an unmatched principal.
severity: minor
resolution: 'Intended: listing is a read like the affordance. Documented on actionOps.list.'
status: addressed
---
