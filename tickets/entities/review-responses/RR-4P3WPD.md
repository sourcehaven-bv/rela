---
id: RR-4P3WPD
type: review-response
title: '"no icon" encoded as empty string is erased by omitempty — indistinguishable from absent, and collides with the meaning the plan says it must not overload'
finding: |-
    The plan's wire-layer design (Approach §4) says `none` clears `item.Icon` to empty, and that this needs no new SPA field because "`icon: ""` already means 'nothing to render' to a `v-if`".

    That is wrong in two compounding ways.

    **1. `omitempty` deletes the signal.** `v1.SidebarItem.Icon` is tagged `json:"icon,omitempty"` (internal/apiwire/v1/responses.go:382). An empty string is omitted from the JSON entirely, so the SPA receives `{label, href}` with no `icon` key. That is byte-identical to what an OLD server sends for an entry it derived no icon for. The SPA therefore cannot distinguish "the author deliberately asked for no icon" from "this field was never populated".

    **2. It re-creates exactly the overload the plan rejected.** The ticket rejects `icon: ""` as the author-facing syntax because empty already means "use the derived icon". But the wire design then uses empty for the OPPOSITE meaning — "render nothing". Empty now means "derive" in YAML and "suppress" on the wire. The one place those two meanings must not be confused is the boundary between them.

    This is not theoretical. `navEntryToSidebarItem` (views_handler.go:347-390) sets `item.Icon` from the kind switch and then applies the override. If `none` maps to `""`, the resulting payload is the same as an entry that fell through the switch with no match — e.g. a malformed entry with no `list:`/`kanban:`/`action:` at all. A future reader of the wire payload cannot tell the two apart, and neither can a test.

    **The plan's own AC 6 cannot be tested as written**: "`Icon: "none"` → wire `Icon == ""`" asserts the field is empty, which is also true for the malformed case, so the test passes without proving suppression happened.
resolution: |-
  Addressed in plan (Approach §4). The sentinel is carried end to end as the literal string `none` — never mapped to empty at any layer. A `NoIcon`/`NO_ICON` constant replaces bare literals. AC 6 restated to assert `Icon == "none"` rather than emptiness, which a malformed entry also satisfies.
severity: critical
status: addressed
---

## Recommended fix

Carry `none` through to the wire as a **distinct value**, and let the SPA decide
how to render it. Two viable shapes:

**Option A (preferred) — keep the sentinel on the wire.**
`navEntryToSidebarItem` passes `"none"` through unchanged. `omitempty` no longer
erases it because the string is non-empty. The SPA renders:

- `icon` absent/empty → nothing (legacy behaviour, unchanged)
- `icon === "none"` → the reserved spacer
- otherwise → `resolveIcon(icon)`

This keeps the author-facing name and the wire value identical, which makes the
whole feature one concept instead of two, and makes AC 6 assertable (`Icon ==
"none"`, not `Icon == ""`).

It also means `resolveIcon("none")` must NOT be reachable — guard it in the
template, and add a test that `isKnownIcon('none') === false` so the name can
never be silently added to the registry as a real glyph.

**Option B — an explicit boolean field.** Add `HideIcon bool
json:"hideIcon,omitempty"`. More wire surface, and `omitempty` bites again for
`false`, but unambiguous.

Option A is recommended: one name, one meaning, end to end.

## Also fix

AC 6 on the ticket must be restated to assert the distinguishing value rather
than emptiness, or it certifies nothing.
