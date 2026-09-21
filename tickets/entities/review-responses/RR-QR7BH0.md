---
id: RR-QR7BH0
type: review-response
title: Omitting the face key on the default face contradicts EntityToTable's always-present convention
finding: 'The plan specifies `face` be omitted from the Lua entity table when the face is the default, mirroring json:"face,omitempty" on the domain type. But EntityToTable (internal/lua/runtime.go:1328-1356) sets every other key unconditionally, including for empty values: mod_time is set to the empty string when UpdatedAt is zero (:1336-1339), and `redacted` is set to an empty table when nothing was hidden, with a comment at :1347-1351 explaining it is ''always present (empty when nothing was hidden)'' precisely so scripts can test it without a nil check. Omitting `face` makes it the only conditional key and forces `e.face or ''default''` at every call site — the pattern the redacted comment exists to avoid. A JSON-wire omitempty tag is a different context: it saves bytes on a serialized payload, whereas a Lua table key costs nothing and its absence is a trap.'
severity: minor
resolution: 'Accepted. The read-side keys are now set unconditionally, empty string for the default face, matching mod_time''s treatment of a zero value (runtime.go:1336-1339) and the always-present rationale recorded for `redacted` at :1347-1351. The json omitempty analogy was removed from the plan: it governs a wire payload where absent keys are a handled case, whereas a missing Lua table key is a nil-index error at the first line that assumes it. Verified no in-tree Lua script iterates entity table keys, so adding a key is unobservable unless asked for.'
status: addressed
---

## Finding

The plan's read-side section says:

> `face` on the entity table and `from_face` on the relation table, both
> omitted (not empty-string) when the face is the default — mirroring the
> `json:"face,omitempty"` on the domain types.

The precedent it cites is from a different context, and the local precedent
points the other way.

**`EntityToTable` sets every key unconditionally**
(`internal/lua/runtime.go:1328-1356`), including when the value is empty:

- `mod_time` is set to `""` when `UpdatedAt` is zero (`:1336-1339`) rather
than omitted.
- `redacted` is set to an empty table when nothing was withheld, and the
comment at `:1347-1351` states the reason directly: it is "always present (empty
when nothing was hidden)" so a script can "both iterate it and test membership".

Adding `face` as the sole conditional key makes the table's shape depend on
data, so every script that touches it needs `e.face or "default"` or a nil
guard. That is exactly the ergonomic problem the `redacted` comment records a
decision against.

**The `omitempty` analogy does not transfer.** `json:"face,omitempty"`
(`internal/entity/entity.go:63`) governs a serialized wire payload, where
omission saves bytes and every consumer already handles absent keys as part of
JSON decoding. A Lua table key costs nothing to include, and absence is not a
handled case — it is an `attempt to index a nil value` at whatever line first
assumes it.

## Counter-argument, and why it does not hold

The plan's stated aim is keeping a faceless script's tables "byte-identical to
today". That is a real concern, but AC 8 already covers the compatibility that
matters (existing calls keep working), and adding a key breaks nothing: verified
that no in-tree Lua script iterates entity table keys (`grep` for `pairs` across
`examples/`, `scripts/`, `tickets/` returns nothing), so no consumer can observe
the extra key except by asking for it.

If byte-identical output for faceless projects is genuinely required, the
consistent form is `face = ""` on the default face — matching `mod_time`'s
treatment of the zero value — rather than omission.

## Suggested resolution

Set `face` unconditionally, using the empty string for the default face, and
follow `mod_time`'s precedent at `internal/lua/runtime.go:1336-1339`. Do the
same for `from_face` on `relationToTable`.

Whichever is chosen, the plan should state it as a decision with its reason
rather than as an inherited JSON tag, and the edge-case table's "`{face = nil}`
same as omitted" row should be checked for consistency with it.
