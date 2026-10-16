---
id: RR-F81DTA
type: review-response
title: 24ch cap truncates names in wide columns
finding: max-width 24ch cuts names even when the column has room.
severity: nit
reason: View tables use auto layout; without a cap one long name widens the column for every row.
status: wont-fix
---
