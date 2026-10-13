---
id: BUG-2TPO2F
type: bug
title: List-context commands receive ungated, unredacted rows on stdin
description: A list-context command reads its input rows from the raw store (commands.go:616 -> listFromStoreByTypes) with no ACL row gate or field redaction; the command output streams back to the caller.
priority: high
status: backlog
---

## Symptom

A command run in a **list** context sends every entity of the list's type to the
command's stdin, without the ACL row gate and without field redaction. The
command's output streams back to the caller over SSE, so a principal who may run
the command sees rows and fields the ACL hides.

`internal/dataentry/commands.go:616`:

```go
entities := listFromStoreByTypes(r.Context(), h.services(), []string{listCfg.EntityType})
```

`listFromStoreByTypes` (helpers.go:626) reads `svc.Store.ListEntities`, the raw
store. The entity context (commands.go:590, `h.visible.untypedAddress` plus
`redactEntity`) and the view context (gated view executor) are gated.
`authorizeCommand` decides whether the command may run, not which rows it may
see.

No ACL test covers the list context.

## Expected

The list context reads through the same gate as `GET /api/v1/<list>`: row gate
by `ReadQuery`, field redaction per row.

## Origin

Found in the raw-store-read audit for TKT-WFS778 (archplan thread th-7cdb1f13).
Confirmed by reading the code; not yet reproduced with a test.
