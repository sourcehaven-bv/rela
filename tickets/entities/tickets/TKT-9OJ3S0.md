---
id: TKT-9OJ3S0
type: ticket
title: Observer backfill for state rows
kind: enhancement
priority: medium
effort: s
status: backlog
---

Design doc §12.7. Observers are not invoked for entities already on disk
(`fsstore.go:69-73`); `backfillBleve` exists for entities. State rows need the
same backfill story so search indexes existing states after an upgrade or after
pointers are enabled on a type.
