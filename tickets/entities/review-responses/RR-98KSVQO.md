---
id: RR-98KSVQO
type: review-response
title: Inaccessible fields are not checked
finding: A field withheld by a storage failure was not treated as hidden so a template title could render in part.
severity: minor
resolution: WholeDisplayTitle falls back to the address when any title property is absent from the row; a withheld field is absent too.
status: addressed
---
