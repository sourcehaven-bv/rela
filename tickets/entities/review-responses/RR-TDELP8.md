---
id: RR-TDELP8
type: review-response
title: max-width 40ch caps legitimately wide columns and is only advisory in table layout
finding: max-width on a cell is a hint rather than a constraint in the CSS table algorithm, so the cap is not reliably load-bearing against the long-URL case it was added for. It also unconditionally caps a legitimately wide prose column while horizontal space sits unused.
severity: significant
resolution: 'Added the reviewer''s targeted rule (td a { overflow-wrap: anywhere }) so an unbreakable URL can break at any character, which addresses the 2111px case directly rather than relying on an advisory cap. Kept max-width 40ch as a secondary bound for non-link content. Confirmed in-browser that the engine treats it as advisory: computed max-width 343px on a cell rendering at 922px.'
status: addressed
---

# Finding

Two objections to the unconditional `max-width: 40ch` on cells:

1. `max-width` on a cell in a `border-collapse: collapse` table is **advisory**
in the CSS table algorithm — engines treat it as a hint, so the mitigation for
the 2111px URL blow-out is not reliably load-bearing.
2. It caps a legitimately wide prose column (one wide column beside three narrow
ones) while horizontal space sits unused.

# Resolution

Took the reviewer's targeted remedy, which attacks the actual cause:

```css
.md-body:not(.milkdown-prose) td a,
.md-table-scroll > table > * > tr > td a {
  overflow-wrap: anywhere;
}
```

A cell holding only a link is almost always a URL, and a URL's min-content width
is the entire string — `break-word` does not shrink it, so the `12ch` floor
cannot either. Letting a *link* break at any character is acceptable (it is a
URL, not prose) and removes the need for the ceiling to be the sole defence.

**Confirmed the advisory claim in-browser**, rather than taking it on trust: the
URL cell reports a computed `max-width` of `343.066px` while actually rendering
at `922px`. The engine is indeed ignoring it.

`max-width: 40ch` is **kept** as a secondary bound for non-link content (a cell
of very long unbroken prose, e.g. a hash or an identifier run). Since it is
advisory it cannot over-constrain a legitimately wide column in practice — the
measurement above shows the engine widening past it when the content requires —
so it costs nothing and helps where the engine does honour it.
