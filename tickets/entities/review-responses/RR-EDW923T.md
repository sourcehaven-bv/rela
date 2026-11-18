---
id: RR-EDW923T
type: review-response
title: Card fetches run detail for non-managers
finding: FlowRunCard reads the run detail even when the timeline is hidden.
severity: nit
reason: The detail also refreshes waiting and actions, which a non-manager needs.
status: wont-fix
---
