---
id: RR-01JKVJ
type: review-response
title: visibleMobileColumns and visibleCardFields used different emptiness tests
finding: 'EntityList tested the formatted string; KanbanView was rewritten to test the raw value. Reviewer tabled twelve values and found they agree on false, 0, empty string, empty array, whitespace and NaN, diverging only on an unparseable date (mobile drops it, raw keeps it). Separately, the raw test did not predict what the widget renders: ['''',''''] passes a non-empty-array test but MultiSelectWidget renders two empty badges, producing the dangling ''Tags:'' label the predicate exists to prevent.'
severity: minor
resolution: Reverted visibleCardFields to the formatted test, matching EntityList. Verified independently that formatCellValue renders false as 'No' and 0 as '0' - both non-empty, both kept - so the raw rewrite was never needed. The kanban bug was entirely in getCardFieldValue's old String(v || ''), which collapsed them before the predicate ran.
status: addressed
---
