---
id: RR-1YR710L
type: review-response
title: Duplicated JSON pointer helper
finding: dataentry jsonPointerEscape duplicates entitymanager.propertyPointer.
severity: nit
reason: Exporting the helper would widen the entitymanager API for four lines; the escape is RFC 6901 and cannot drift.
status: wont-fix
---
