---
id: RR-U04YVMZ
type: review-response
title: 'Untested combinations'
finding: 'No Sidebar test combines a permission-hidden and a phone-hidden row; detailActions does not test labels that differ only by case or accent.'
severity: minor
reason: 'The sidebar logic is an AND of two sets that are each tested, and the server test covers the combination; the label comparison predates this change.'
status: wont-fix
---
