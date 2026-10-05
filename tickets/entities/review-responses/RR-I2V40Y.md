---
id: RR-I2V40Y
type: review-response
title: anyFaceOf could reuse familyRows
finding: anyFaceOf and familyRows both read the family by id.
severity: nit
reason: anyFaceOf serves other write paths with different needs; merging them widens this bug fix into a refactor of unrelated callers.
status: wont-fix
---
