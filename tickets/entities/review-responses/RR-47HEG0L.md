---
id: RR-47HEG0L
type: review-response
title: Operator confirm text ignored for batch actions
finding: 'A batch action''s confirm: text was replaced by the generic question.'
severity: minor
resolution: The confirm uses the action's confirm text when it is a string and the generic question otherwise; tested.
status: addressed
---
