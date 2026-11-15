---
id: BUG-2TPO2F
type: bug
title: List-context commands receive ungated, unredacted rows on stdin
description: A list-context command reads its input rows from the raw store (commands.go:616 -> listFromStoreByTypes) with no ACL row gate or field redaction; the command output streams back to the caller.
priority: high
effort: s
why1: 'The list branch of buildCommandInput read its rows with listFromStoreByTypes. That helper drained svc.Store.ListEntities: the raw store with no ACL row gate and no redactEntity call.'
why2: authorizeCommand decides whether a principal may run the command. The list branch treated that check as enough and never asked which rows the principal may read. The entity and view branches had moved onto gated readers (DEC-ZBI39P); the list branch kept the raw helper.
why3: The raw list helper sat on the unpaged-read allowlist as cost debt. Its entry judged how much it read and not whether the rows it returned left the process ungated. So the guard that named the read never raised the visibility question.
why4: No ACL test drove a list-context command. The command ACL tests covered the entity context only; the golden parity case recorded the leak as a known gap instead of failing on it.
why5: Read-side gating lives in visibility wrappers injected at wiring; but dataentry handlers can still reach svc.Store directly. A new output path stays ungated unless its author routes it through the gate by hand.
prevention: 'TestCommandExec_ListContextIsGatedAndRedacted pins the row gate and field redaction of the list-context payload; the golden parity case BUG-2TPO2F-command-list-input pins it per principal over HTTP. The raw helpers listFromStoreByTypes and listAllFromStore are deleted with their unpaged-read allowlist entries so the payload cannot fall back to a raw read. Fixed in 7b6c0c8ea (PR #2247; TKT-F2EJQY8). TKT-D3RAB0E moves the remaining scopedHeaders callers onto paged reads.'
started: "2026-11-14"
completed: "2026-11-14"
status: done
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
