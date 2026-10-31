---
id: RR-U8F6O6X
type: review-response
title: Comments and pile navigation from the old entity stay
finding: loadComments and loadScopeNav ran inside loadView while servedRef still named the old row; when a refresh landed the new row, nothing reloaded them.
severity: significant
resolution: A watch on servedRef reloads both when the row on screen changes; loadView reloads them only for the same row.
status: addressed
---
