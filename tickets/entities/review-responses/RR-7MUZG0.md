---
id: RR-7MUZG0
type: review-response
title: One store error now empties every later row's relation cells
finding: A store error partway through the single batched read truncates the edges of every row after that point and the export still renders a complete-looking table. The per-row loop limited the damage to one row.
severity: minor
resolution: The export now reads through pageRelationsDir which returns the error. The renderer fails and the client gets the generic 500 export_failed. TestExport_List_EdgeReadFaultFailsTheExport pins it and checks that neither rows nor the error text leak.
status: addressed
---
