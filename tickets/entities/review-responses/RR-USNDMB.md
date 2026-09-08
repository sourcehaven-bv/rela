---
id: RR-USNDMB
type: review-response
title: A nil hole in mail.render rows silently emitted an empty <tr></tr>
finding: 'In internal/lua/mailrender.go, parseRenderRows delegated each row slot to stringArray, which maps LNil to (nil, nil) — its documented "omitted optional array" case. A nil hole in the rows array therefore appended a zero-cell row, rendering a bare <tr></tr> between the real rows with no error. Verified: mail.render{sections={{columns={"a","b"}, rows={{"x","y"},nil,{"z","w"}}}}} produced an empty row. This was the one place the binding abandoned its otherwise-strict parsing.'
severity: significant
resolution: 'parseRenderRows now rejects LNil at the row index explicitly, before delegating to stringArray, with an error naming the position. The rule that leaked was context-dependent: absence is meaningful for `columns` and `links` (both genuinely optional) but not for a row SLOT, where a row is an element rather than an option. Lua''s `#` is also undefined on a table with a hole, so the length cannot be trusted either — failing loudly is the only honest answer. Pinned by TestMailRender_RejectsNilHolesInArrays, which also covers sections and columns (both already rejected holes), plus TestMailRender_SparseLinksLeaveRowsUnlinked asserting the links case stays permissive.'
status: addressed
---

## Finding

Reported by cranky-code-reviewer, verified by reproduction before fixing.

`parseRenderRows` (`internal/lua/mailrender.go`) looped `1..tbl.Len()` and
handed each slot to `stringArray`. That helper documents `LNil` as "this
optional array was omitted" and returns `(nil, nil)` — correct for `columns` and
`links`, wrong for a row.

Reproduced end to end:

```lua
mail.render{subject="S", sections={{columns={"a","b"}, rows={{"x","y"},nil,{"z","w"}}}}}
```

rendered `<tr></tr>` between the two real rows. No error raised.

Everywhere else the binding is strict — a numeric cell, a hole in `sections`,
and a hole in `columns` all raise. This was the single inconsistency, and it
produced silently wrong output rather than a loud failure.

## Resolution

`parseRenderRows` now checks for `LNil` at the row index and returns an error
naming the position, before `stringArray` gets a chance to apply its
optional-array semantics.

The comment at the fix records *why*, since the helper's behaviour is right in
its own context: absence is meaningful for an optional array, meaningless for an
element slot. It also notes that Lua's `#` is undefined on a table with a hole,
so the iteration count is itself unreliable — which is the deeper reason to
refuse rather than to paper over.

Tests added:

- `TestMailRender_RejectsNilHolesInArrays` — holes in `rows`, `sections` and
`columns` all raise, each with its own expected message.
- `TestMailRender_SparseLinksLeaveRowsUnlinked` — the complement, pinning that
a short `links` array is NOT an error: trailing rows simply render unlinked,
which is documented `buildSection` behaviour. Without this, a later reader might
"fix" the asymmetry in the wrong direction.
