---
id: RR-J8O28W
type: review-response
title: First-element loop to read a family's type
finding: resolved() iterated a map and broke after one element to read Type.
severity: nit
resolution: The Type field and that loop are gone. ReadableTypes keeps one such loop, with a comment on why any header answers.
status: addressed
---
