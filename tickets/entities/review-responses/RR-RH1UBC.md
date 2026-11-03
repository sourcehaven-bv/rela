---
id: RR-RH1UBC
type: review-response
title: Rolling deploy can render recorded refs as plain ids
finding: A process booted with a property changed from ref to string, before the type had a record, renders refs another process later recorded as plain strings, exposing hidden entity ids.
severity: minor
resolution: The renderer treats a property as a ref when the config or the store's type record says so; Local caches recorded refs and refreshes on config change or ErrTypeShapeConflict. TestRender_RecordedRefWinsOverConfig.
status: addressed
---
