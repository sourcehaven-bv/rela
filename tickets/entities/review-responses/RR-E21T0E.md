---
id: RR-E21T0E
type: review-response
title: Grouped list can silently drop the busy column
finding: 'visibleListColumns (EntityList.vue:719-725) removes the column that shows the group property, and toTableColumns then drops the first remaining column as the title (tableColumns.ts:68). titleColumn is still taken from the unfiltered list (EntityList.vue:784-786). When column 0 shows the grouped property and the busy column is column 1, the busy column becomes the first visible column and is sliced away, while the list still sends busy and pays for the read. This bug exists today for any column, but the busy column turns it into a missing feature with no error. Fix: slice the title column off before filtering, or filter by identity against titleColumn. Add an EntityList.busy.test.ts case for a list grouped by its first column.'
severity: minor
resolution: ADDRESSED step 9 takes the title column off before filtering the grouped-property column; new AC 14 with an EntityList.busy.test.ts case for a list grouped by its first column
status: addressed
---
