---
id: RR-PSPVOF
type: review-response
title: RlBusyIndicator is read twice by screen readers
finding: 'Plan step 8 renders the visible label ("Planning") and a visually hidden title ("Agent run: Planning") side by side. A screen reader reads both: "Planning Agent run: Planning". The plan should mark the visible text aria-hidden and keep only the hidden full text, or make the hidden part only the prefix ("Agent run:"). It should follow LeaseBadge.vue''s pattern and assert the accessible name, not the presence of both spans, in KanbanView.cardBusy.test.ts.'
severity: minor
resolution: The visible text is aria-hidden and only the hidden full text is read, following LeaseBadge.vue; KanbanView.cardBusy.test.ts asserts the accessible name.
status: addressed
---
