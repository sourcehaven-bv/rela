---
id: RR-E911EP
type: review-response
title: Baseline drops targets whose replay did not finish
finding: 'Cranky review: a cursor omitted unread targets, so the next call baselined them silently and lost a change in between.'
severity: minor
resolution: result returns the tool error watchIncomplete when any target has no digest. TestWatch_IncompleteBaselineIsAnError pins it; docs say to retry.
status: addressed
---
