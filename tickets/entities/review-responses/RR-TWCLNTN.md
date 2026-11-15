---
id: RR-TWCLNTN
type: review-response
title: Configure Automations editor left dead
finding: Dropping automations.* from the allowlist leaves the view, routes, models and reference warnings working on an empty list.
severity: significant
resolution: 'Plan: remove the Configure automations view and routes in this PR with a note pointing to flows.yaml; porting the editor is a follow-up ticket.'
status: addressed
---
