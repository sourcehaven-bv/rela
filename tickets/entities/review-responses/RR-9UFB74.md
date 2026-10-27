---
id: RR-9UFB74
type: review-response
title: Rung numbers will collide with the mentions slice
finding: The datamigration timestamp rule covers project data migrations, not the schema ladders, so 0035 and v28 are right. TKT-6RXAC0 (mentions) will also add a rung. Land the slices in order and renumber on rebase. The pg prefix-unique test catches a duplicate 0035; confirm that the sqlite ladder test also rejects two rungs with the same `to`.
severity: nit
resolution: 'Noted: land in order, renumber on rebase. Confirmed TestMigrationLadderIsWellFormed requires contiguous to versions, so two rungs to v28 fail it.'
status: addressed
---
