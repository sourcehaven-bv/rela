---
id: RR-MC03GX
type: review-response
title: Duplicate delete-fence subquery and ordering differences between backends
finding: sqlite repeated the delete-fence subquery for lvc_dirty; the two backends order candidates differently; WHERE-only scan variables were commented awkwardly.
severity: nit
resolution: lvc_dirty and its scan variables are gone with the column gate. The ordering difference predates this bug and no longer affects correctness, since only changed rows are candidates.
status: addressed
---
