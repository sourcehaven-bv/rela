---
id: TKT-L1ZXTQU
type: ticket
title: 'Docs: query scopes with not/or lower into the store'
kind: docs
priority: low
effort: xs
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

GUIDE-metamodel says a list query scope lowers only when it is a plain
conjunction, and that `not`, `or` and ordered comparisons are evaluated after
the store read. Since TKT-I39Y7N4 and TKT-8HY05WB, scopes and list `condition:`
lower through `queryplan.LowerCondition`, so `not related(...)` runs in the
store query on postgres and sqlite (NOT EXISTS). Correct the text.
