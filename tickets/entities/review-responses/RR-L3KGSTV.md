---
id: RR-L3KGSTV
type: review-response
title: Property triggers do not fire on create
finding: 'docs/flows.md:716: a property trigger does not fire on create. A ticket created with status=planning gets no checklist and no started; same for a bug created in-progress and a docs-checklist created in-progress.'
severity: significant
resolution: 'Added test cases pinning the current behaviour (create-in-status creates nothing; docs-checklist created in-progress gets no started) and a note for agents in CLAUDE.md: create in backlog or ready and then move the status.'
status: addressed
---
