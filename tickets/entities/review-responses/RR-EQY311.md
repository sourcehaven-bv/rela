---
id: RR-EQY311
type: review-response
title: No width for the busy column
finding: The plan gives the busy TableColumn no width, so it takes the table default --rl-cell-width. 'Planning · 3h +1' plus the dots may truncate the label in a normal table or waste space. Pick a width in toTableColumns (or EntityList) for busy columns and show it in the BusyColumn story.
severity: nit
resolution: ADDRESSED toTableColumns sets width 160 on a busy column (step 8, AC 7 and tableColumns.test.ts), shown in the BusyColumn story
status: addressed
---
