---
id: RR-NWBAR5
type: review-response
title: A present-but-non-table opts argument has no specified behaviour, reintroducing the silent-drop in the argument being added
finding: 'The plan specifies rejecting unknown KEYS inside the opts table but says nothing about a present-but-non-table opts ARGUMENT. Verified against gopher-lua directly: GetTop() counts explicit trailing nils, so the binding CAN distinguish an omitted 5th arg (GetTop=3) from an explicit nil (GetTop=5). The hazard: rela.create_entity(''policy'', props, ''body'', nil, ''draft'') — a caller passing the face as a bare string, which is exactly what someone will do — arrives as GetTop=5 with arg5 of type string. If the helper is written as the natural `ls.Get(5).Type() != lua.LTTable -> ignore`, that call is SILENTLY IGNORED and the create then fails with a confusing face_required, or on a faceless type succeeds on the wrong face. That is BUG-HC6I2T''s exact silent-drop shape, occurring in the very argument this ticket exists to add. The plan''s own edge-case table also justifies the nil case via a stated impossibility that does not hold.'
severity: critical
resolution: 'Accepted, and the gopher-lua behaviour independently re-verified with a probe: GetTop() does count explicit trailing nils, so absent (GetTop=3) and explicit nil (GetTop=5) are distinguishable, and a bare string in the opts slot arrives as GetTop=5 with a string type. Argument handling is now specified exhaustively in the plan''s Approach item 3: absent -> no face; explicit nil -> same as absent; a table -> parse with key and type rejection; any other type present -> RAISE. The string form create_entity(t, p, c, nil, ''draft'') is called out as the mistake a real author makes and is pinned in AC 5 and the edge-case table. The nil row''s incorrect justification was corrected.'
status: addressed
---

## Finding

The plan is careful about unknown *keys* inside the opts table and says nothing
about a present-but-non-table opts *argument*.

## Verified against gopher-lua

I probed the actual runtime rather than reasoning about it:

```
probe("a", {}, "c")                       -> GetTop=3 arg4=nil    arg5=nil
probe("a", {}, "c", "id")                 -> GetTop=4 arg4=string arg5=nil
probe("a", {}, "c", nil, nil)             -> GetTop=5 arg4=nil    arg5=nil
probe("a", {}, "c", nil, {face="draft"})  -> GetTop=5 arg4=nil    arg5=table
probe("a", {}, "c", nil, "draft")         -> GetTop=5 arg4=nil    arg5=string
```

Two consequences.

**First, the plan's stated impossibility is not one.** Its edge-case table
justifies "`nil` passed explicitly as the opts table | Same as omitted" on the
basis that Lua cannot distinguish them. `GetTop()` counts explicit trailing
nils, so the binding *can*: omitted gives `GetTop=3`, explicit nil gives
`GetTop=5`. The desired behaviour is still "treat both as absent" — the outcome
is right, the reasoning is wrong, and reasoning that does not hold is the kind
that gets extended to a case where it matters.

**Second, and the real hazard** — the last probe line:

```lua
rela.create_entity("policy", props, "body", nil, "draft")
```

A caller passing the face as a bare string, which is precisely what a script
author who half-remembers the signature will write. It arrives as `GetTop=5`
with `arg5` of type `string`.

If the helper is written the natural way — `if ls.Get(5).Type() != lua.LTTable {
return }` — that call is **silently ignored**. On a faced type the create then
fails with a confusing `face_required` naming an argument the caller believes
they passed; on a faceless type it succeeds on the wrong face.

**This is BUG-HC6I2T's exact shape** — a face that does not reach the write —
reproduced in the very argument this ticket exists to add, and the plan's
unknown-key rule does not cover it because the value never becomes a table to
have keys checked.

## The precedent to copy, precisely

The plan cites `relationQuery` but not the guard clause that matters
(`internal/lua/runtime.go:1832`):

```go
if s.GetTop() < 1 || s.Get(1).Type() != lua.LTTable {
```

a `GetTop`-plus-type-check pair. `luaUpdateEntity` uses the same idiom at
`:1767` and `:1771`. There are two in-tree precedents and the plan cites neither
precisely enough to disambiguate the failure above.

## Fix

Specify explicitly, for both bindings:

- **Absent** (`GetTop() < pos`) → no face, current behaviour.
- **Explicit `nil`** → same as absent.
- **A table** → parse it; reject unknown keys and wrong-typed values as
planned.
- **Any other type present at that position** → **raise**, naming the expected
shape.

Pin the last case with a test — specifically `{..., nil, "draft"}`, the string
form, since that is the mistake a real author makes. Then correct the edge-case
table's justification for the nil row.
