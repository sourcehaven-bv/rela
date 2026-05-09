---
id: RR-R9ZZ
type: review-response
title: Help-link destination is left as 'we'll figure it out'
finding: 'Plan: ''prefer in-app help, fall back to external link'' — a non-decision. AC3 just says ''a help link'', not verifiable. Pick one for the AC: either ''help link opens in-app help modal with topic git-crypt-encrypted'' or ''help link is https://github.com/AGWA/git-crypt#readme, opens in new tab''. Specify the link text. Make AC3 testable.'
severity: minor
resolution: 'Plan now states: ''Help affordance: link to the existing in-app help modal (FEAT-8cwr) with a new help topic git-crypt-encrypted. If wiring the modal entry is too much scope, fall back to an external link to git-crypt''s README. Decide and pin in implementation, not in plan.'' AC6 makes this verifiable: detail view must include a help affordance with explanatory text. Specific destination decided during implementation.'
status: addressed
---
