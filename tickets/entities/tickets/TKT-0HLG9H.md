---
id: TKT-0HLG9H
type: ticket
title: count_related() predicate lowered into store queries
kind: enhancement
priority: low
effort: l
status: backlog
---

## Description

Follow-up of TKT-6I2OPG (read-time roll-ups). Add a `count_related(path, where)`
predicate function so filters and conditions can compare a related count,
lowered into `store.GraphQuery` the way `related(...)` is, so lists can filter
on it without a per-row fold.
