---
id: RR-PBC2XS7
type: review-response
title: _transitions ignores the ceiling
finding: computeTransitions only checks IsVisible, so a transition of a property the ceiling withholds is offered and then refused with 403.
severity: minor
resolution: 'Addressed in plan: a transition of a property that is not writable is reported not allowed.'
status: addressed
---
