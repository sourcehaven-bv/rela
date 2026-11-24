---
id: RR-1GFHGJV
type: review-response
title: Other query surfaces and body_tasks
finding: Kanban, view conditions or scopes might accept a body_tasks roll-up in a filter or sort.
severity: significant
resolution: 'Checked: only the list pipeline queries roll-ups (queriedRollups); conditions naming a roll-up fail to load and boards do not sort on roll-ups.'
status: addressed
---
