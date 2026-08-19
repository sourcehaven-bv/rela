---
id: TKT-OX8Q3E
type: ticket
title: pgstore rename should emit EntityRenamed, not delete+put
kind: refactor
priority: medium
effort: s
status: backlog
---

Design doc §12.4. pgstore rename emits `notifyDelete(oldID)` +
`notifyPut(renamed)` (`pgstore/entity.go:484-485`); no `notifyRenamed` method
exists on pgstore at all (only fsstore has one). Harmless today only because the
pg search backend is a no-op observer — any observer with real behaviour sees a
transient delete on rename. Align with the store contract callback (`store.go`
EntityRenamed) before states multiply observer traffic.
