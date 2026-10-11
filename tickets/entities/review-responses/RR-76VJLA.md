---
id: RR-76VJLA
type: review-response
title: Draft edits inside list items were silently dropped
finding: ensurePath returned undefined for any path with a list index, so setAt/appendAt did nothing for rules, automations, board columns, list columns, dashboard cards, form fields and nav items.
severity: critical
resolution: ensurePath descends into existing list items (keeping $i) and never creates one. Unit tests on list paths; e2e test saves a board column header.
status: addressed
---
