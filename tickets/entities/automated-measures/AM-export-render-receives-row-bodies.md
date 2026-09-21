---
id: AM-export-render-receives-row-bodies
type: automated-measure
title: "A list export's render script receives real row bodies; the built-in table still reads none"
kind: test
location: internal/dataentry/export_list_render_test.go
status: active
description: "Pins BUG-RGVKRV in BOTH directions. TestExport_List_RenderOverrideReceivesRowBodies asserts a Lua export_render: script receives each row's real body, so a future content-free narrowing cannot silently re-empty it. TestExport_List_BuiltinTableReadsNoBodies asserts the built-in column table still reads zero bodies, so the fix cannot regress BUG-SDMD6O's retention bound by loading bodies for everyone."
---

Pins BUG-RGVKRV.

## Why two assertions, not one

The fix for BUG-RGVKRV and the fix for BUG-SDMD6O pull in opposite directions.
One says *this consumer must receive bodies*; the other says *a list request
must not read bodies it will not render*. A measure asserting only the first
could be satisfied by loading every body again, re-introducing the retention
defect that [[AM-list-endpoint-bounded-retention]] exists to prevent.

So both directions are pinned:

- **`TestExport_List_RenderOverrideReceivesRowBodies`** — an `export_render:`
  script reads `row.content` and gets the real body. Fails with
  `row 0 content = "", want "the body of TKT-1"` if the `loadBodies` seam is
  removed. Verified by mutation.
- **`TestExport_List_BuiltinTableReadsNoBodies`** — the built-in column table
  path reads **zero** bodies, counted via `storetest.BodyWatch`. Fails if the
  fix is widened to load bodies unconditionally.

Together they state the real invariant: bodies are loaded for the consumer that
renders them, and for no one else.

## Known gap

The shared `fakeScriptEngine` records row ids only, which is precisely why the
original regression was invisible to the existing override tests. These two
tests assert against the body values directly rather than through that engine.
Widening `fakeScriptEngine` to capture what a script receives is recorded as a
follow-up on BUG-RGVKRV (P3), and would retire this class of blind spot beyond
this one bug.
