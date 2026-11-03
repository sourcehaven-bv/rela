---
id: RR-8SN3XE
type: review-response
title: canSortColumn returns true for relation columns
finding: 'models.ts:724 hides the Sortable switch only for busy and face columns, and its doc reasons that they "name no property". A relation column also names no property, and the SPA never sorts it (tableColumns.ts toTableColumns: sortable requires column.property). The page therefore still offers a Sortable switch on a relation column that does nothing, and models.test.ts pins that as true. Fix: return !!c.property and update the test row for blocks to false.'
severity: minor
resolution: 'ADDRESSED canSortColumn returns !!c.property, so relation columns lose the no-op Sortable switch too (cheap and safe: the SPA never sorts them); models.test.ts row for blocks is now false.'
status: addressed
---
