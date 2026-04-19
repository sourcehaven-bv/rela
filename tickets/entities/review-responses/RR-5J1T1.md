---
id: RR-5J1T1
type: review-response
title: Historical references to old singular folder paths in tickets/
finding: 'Three historical documents in tickets/ reference the old singular paths: tickets/entities/review-responses/RR-MR18.md:7 and tickets/entities/planning-checklists/PLAN-KP5I.md:219 reference docs-project/entities/guide/GUIDE-data-entry.md; tickets/entities/planning-checklists/PLAN-TYI7.md:70 references internal/lua/rela-docs/entities/concept/lua-scripting.md (also a fictional path). These are closed/historical artifacts. Grepping for the old path in the future will produce misleading hits, but nothing is runtime-broken.'
severity: minor
reason: The three references are in closed/historical documents (a closed review-response and two completed planning-checklists). Editing closed historical artifacts would falsify the record of what was written at the time. Future grep hits are a mild annoyance but not a correctness issue. Accepting the cost.
status: wont-fix
---
