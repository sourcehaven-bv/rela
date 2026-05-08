---
id: RR-2LTXT
type: review-response
title: resolve_refs flatten-rewrite-wrap destroys the new structure it just gained
finding: Plan keeps resolve_refs working via flatten→rewrite→wrap-as-text-leaf. But that turns every touched paragraph back into a single flat text node, losing the link/code/raw_html structure the refactor introduced. So 'entity_refs → resolve_refs → render' loses link URLs even though the refactor advertises link preservation. Either fix resolve_refs in this PR or adjust the value-prop ACs (4, 5) to say 'unless touched by resolve_refs'.
severity: significant
resolution: resolve_refs rewritten in this PR to walk the inline tree directly. Skips code_span, raw_html, autolink, image inlines and link URL/title slots. Recurses into emphasis/strong/strikethrough/link containers. Rewrites only `text` leaves, splitting at match boundaries. Run-length backtick scanner and Unicode-boundary code from TKT-LXYHQ are deleted. Pinned in AC15, AC16, AC17.
status: addressed
---

# Finding

The plan's `resolve_refs` migration strategy:

1. Flatten each paragraph's `inlines` to a string via
`flattenInlines`.
2. Run the existing scanner over that string.
3. Wrap the result as a single `{type="text", text=...}` inline.

The structural information (links, code spans, raw HTML) is gone after step 3.
So a script that does `entity_refs → resolve_refs → render` on entity content
with a link inside loses the link's URL — even though the parse step preserved
it.

This silently undoes the value of AC4 (link round-trip) and AC5 (raw HTML
round-trip) for any document that gets `resolve_refs`'d.

# Resolution

Two options:

1. **Fix `resolve_refs` in this PR.** Walk `inlines` directly:
skip `code_span` and `raw_html` and the link's URL/title; recurse into
containers (link inlines, emphasis, strong, strikethrough); rewrite only the
`text` field of `text` leaves. This is the work the plan said is "out of scope,
follow-up ticket".
2. **Keep it out of scope, but be honest.** Update AC4 to say
"links round-trip *for documents not processed through `resolve_refs`*", and add
a follow-up ticket reference.

(1) is the principled choice but bigger. The plan as written took (2) implicitly
without saying so.

Recommend (1) for this PR. The follow-up ticket gets cancelled, the new
`resolve_refs` is ~80 lines simpler than the existing one, and the value-prop is
honest. Cost: ~1 extra hour and adds another ~50 lines to the diff.

If we stay with (2), update AC4/AC5 to scope the claim and add a note to the
docs.
