---
id: RR-FRC1SP
type: review-response
title: 'The forced-colors fallback lost the cascade and did nothing — the ticket''s headline fix was inert'
finding: 'The global rule was `:focus-visible { outline: 2px solid Highlight }` — specificity (0,1,0). The declarations it exists to override are component rules like `input:focus { outline: none }` at (0,1,1), which Vue''s scoped `[data-v-x]` attribute lifts further still. The component rule wins, so the outline stayed suppressed; forced-colors then drops the box-shadow as designed, and the control ends up with NO focus indicator at all — exactly the bug the file was added to fix. `@layer rela` does not help: layers order rela-vs-operator, not rela-vs-rela. A keyboard user in Windows High Contrast would have tabbed through every text, number, date, select and textarea widget seeing nothing, identical to develop.'
severity: critical
resolution: 'Added `!important` to both declarations inside the forced-colors block. Verified in real Chrome that the fallback then wins against a scoped `input[data-v-x]:focus { outline: none }` inside `@layer rela` (outline resolves to the fallback''s value instead of `none`). Inverted the test that had asserted `!important` was ABSENT — it was pinning the bug in place — so it now asserts the declaration is present, with the specificity arithmetic recorded as the reason.'
status: addressed
---

Confirmed independently before fixing rather than taken on the reviewer's word.
A CSS engine resolves the component rule to `outline-style: none`, i.e. the
global fallback is defeated:

```
outline-style : none
=> component rule wins? YES - global fallback DEFEATED
```

jsdom turned out not to honour `!important` in its cascade, so it could not
adjudicate the fix; the confirmation was done in Chrome against the real
`@layer rela` wrapper.

**The `!important` reasoning in the original file was backwards.** It refused
`!important` to "keep operator custom.css able to win" — but a layered
`!important` already outranks an unlayered operator `!important`
(`docs/customisation.md` documents this as permanent), so the property it
claimed to protect was never real. And forced-colors is an accessibility
floor, not a skin: an operator should not be able to accidentally remove the
only focus indicator a High Contrast user gets. Documented at the declaration.

**The deeper lesson is [[RR-FRC2GD]]**: seven green guard tests sat over this.
