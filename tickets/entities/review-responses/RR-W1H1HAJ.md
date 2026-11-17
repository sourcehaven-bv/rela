---
id: RR-W1H1HAJ
type: review-response
title: No-text search reads entity bodies
finding: typePages uses ListEntitiesPage, which returns full entities; collection reads should be content-free.
severity: significant
resolution: Filed as TKT-Q918DFR. The old listAll read bodies too, so this is not a regression; it needs a new keyset header read on every backend.
reason: Filed as TKT-Q918DFR. The old listAll read bodies too, so this is not a regression. The fix needs a new keyset-paged header read on every store backend, which is store work outside this ticket's scope.
status: deferred
---
