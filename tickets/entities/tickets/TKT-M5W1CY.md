---
id: TKT-M5W1CY
type: ticket
title: 'Scheduled Lua tasks get row gating but no field-level visible: redaction (appbuild has no affordance resolver)'
kind: enhancement
priority: medium
status: backlog
---

`appbuild.ScheduledLuaWriteDeps` calls `luaWriteDepsFor(nil)` — a nil redactor,
which `scriptEntityReader` substitutes with `visibility.NopRedactor`. Scheduled
Lua tasks therefore get **row** gating but no field-level `visible:` redaction.

This is the pre-existing limitation already documented in
`ScheduledLuaWriteDeps`' own godoc as RR-7408F5:

> appbuild has no affordance resolver, so scheduled jobs get ROW gating
> only — **field-level `visible:` redaction does NOT apply here**.

## Why file it now

TKT-FJ6END made the gap *observable*. A scheduled script can now call
`entity:is_redacted(name)`, and on this path it always returns `false` — not
because the principal may see the value, but because no field policy was
evaluated. Since the task IS row-gated, an operator reasonably concludes ACL is
in force and reads `false` as "allowed".

Reproduced during that ticket's manual verification: a task running as a
principal whose role granted `visible: [title]` on `person` read `salary` in
full with `redacted_set=[]`.

Documented in the meantime — `docs/lua-scripting.md` carries a runtime table
stating the scheduler always reports `false`.

## Why it matters

The scheduler is the runtime most likely to feed entity data into an LLM prompt
(that is the DEC-O59WM4 motivation for ACL-binding script reads at all). Row
gating bounds *which* entities reach a prompt, which is the larger half — but a
job whose identity may read `person` currently receives every property,
including ones a human with the same role would have redacted in the UI.

## Approach sketch

Wire an affordance resolver into `appbuild` so `luaWriteDepsFor` can be handed a
real `visibility.FieldRedactor` instead of nil. Note `appbuild.go:262`
deliberately substitutes `NopRedactor` for a nil redactor because its callers
legitimately have no resolver — that substitution is the thing to remove, and
doing so is a **change in ACL enforcement scope**: jobs that today read hidden
fields would stop. That needs its own review, which is why it was not folded
into TKT-FJ6END.

Consider also whether `is_redacted()` should distinguish "no policy evaluated"
from "nothing redacted" once this lands; TKT-FJ6END decided against three-way
branching because "unevaluated" is a property of the runtime rather than of an
entity, and closing this gap removes the main case where that distinction would
have mattered.
