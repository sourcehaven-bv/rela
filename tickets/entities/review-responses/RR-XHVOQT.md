---
id: RR-XHVOQT
type: review-response
title: Rows that share an id get no edges from pageRelations
finding: pageRelations skipped duplicate ids so only the first row with an id received edges. Today InWorld returns one row per id so this cannot happen but nothing stated or enforced it. Raised by both the code and the security reviewer.
severity: minor
resolution: pageRelationsDir maps each id to all its row indexes so every row with that id receives the id's edges. TestPageRelationsDir covers two rows with one id.
status: addressed
---
