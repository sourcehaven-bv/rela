---
id: RR-SZTE2ZT
type: review-response
title: Held deletes were re-sent and counted against the guard
finding: Rows held as local_deleted were fetched and deleted again every run and counted in max_gone and the 24 h window.
severity: significant
resolution: guard leaves held local_deleted rows out of sends unless resolved to local; TestDeletes_HeldDeleteWaits.
status: addressed
---
