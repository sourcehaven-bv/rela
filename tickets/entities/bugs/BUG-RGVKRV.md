---
id: BUG-RGVKRV
type: bug
title: 'List export with an export_render: script receives an empty row.content for every row'
description: |-
    A list export whose view declares `export_render:` reaches `row.content` from Lua. Since collection reads became content-free, every row arrives with the empty string. The export still returns 200 and still emits a document, so the loss is silent — only the bodies are gone. The built-in column table is unaffected: it renders properties and never reads a body.
priority: high
effort: s
why1: A list export rendered by a Lua `export_render:` script emits a document whose every row body is the empty string.
why2: The export path builds its rows from the shared collection read, which became content-free so a list page would stop retaining every body of the type.
why3: The content-free change was made for the list-rendering path, where no surface renders a body; the export override path reads the same rows but DOES render bodies, and was not separated from it.
why4: The two paths were not distinguished because they differ only in what the CONSUMER does with a row, not in how the row is fetched — a built-in table renders columns, a Lua script may reach any field. The fetch is shared; the requirement is not.
why5: 'Systemic: a performance narrowing was applied at a shared fetch seam by reasoning about the callers known at the time. Removing data from a shared read is a silent, type-safe change for any caller that merely stops seeing it — an empty string is a valid string. Nothing forced the change to enumerate its consumers, and the one consumer with a different requirement degraded quietly instead of failing.'
prevention: 'P1 (implemented): a `loadBodies` seam on the export handler refills bodies on the override path only, after the ACL scope, field redaction and the cap — so at most listExportCap bodies, all already through every gate. P2 (implemented): AM-export-render-receives-row-bodies pins both directions — the override path receives real bodies, the built-in table still reads none — so a future narrowing cannot re-empty one without failing the other. P3 (not done, follow-up): the shared fakeScriptEngine records row ids only, so any assertion about what a script RECEIVES is invisible to it; widening it would retire a whole class of blind spot beyond this bug.'
status: backlog
---

## Symptom

A list view declaring `export_render:` exports a document in which every row's
body is empty:

```
row 0 content = ""   (want "the body of TKT-1")
row 1 content = ""   (want "the body of TKT-2")
```

The request returns **200** and a document is produced. Nothing errors, nothing
warns. Only the bodies are missing.

## Scope

- **Affected:** list export via a Lua `export_render:` override, which may
  reach `row.content`.
- **Not affected:** the built-in column table, which renders declared columns
  and never reads a body.

## Provenance

Found while pinning BUG-SDMD6O, not by a report. BUG-SDMD6O's own retention
defect was already fixed by TKT-1U8XYN; building the regression pin for it
surfaced this instead.

Confirmed against `bb8d3a144~1`, where the same assertion passes: this is a
regression, not a surface that never worked.

## Why the existing tests missed it

The override path had tests, and they passed. The shared `fakeScriptEngine`
records row **ids** only, so rows arrived in the right order, with the right
ids, and every body empty — indistinguishable from correct at the assertion
surface the tests used.

## Fix

`loadBodies` on the export handler, called on the override path only and
positioned after the ACL scope, field redaction and the export cap, so it can
load at most `listExportCap` bodies and only rows that already passed every
gate. The built-in table path is untouched and stays content-free.
