---
id: RR-0SIJ3GW
type: review-response
title: Missing tests for reload, unreadable type and chosen-channel budget
finding: No test moved a type between blocks on a config reload, and the budget test covered only single mode.
severity: minor
resolution: 'Added: follows a reload that moves a type to the other block; holds at most two live keys per block with a channel chosen from a list. The unreadable-type case is now documented behaviour (heading follows configuration).'
status: addressed
---
