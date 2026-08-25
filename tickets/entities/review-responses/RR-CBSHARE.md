---
id: RR-CBSHARE
type: review-response
title: 'RelationCards hand-rolled a duplicate checkbox — consolidated onto the shared CheckboxWidget'
finding: 'RelationCards.vue styled its own `.inline-edit-checkbox`, a copy of CheckboxWidget''s CSS. The copies had already drifted (the widget gained a theme-following focus ring and a corrected tick position; RelationCards kept the hardcoded indigo ring and the off-centre tick), so the tick fix in RR-CBTICK9 left the inline relation editor visibly wrong. Originally deferred as RR-CBLEV8 on the grounds that it touched a working component outside this ticket''s acceptance criteria; the user overrode that, correctly — shipping a fix that lands on one of two identical surfaces is worse than the scope cost of doing both.'
severity: minor
resolution: 'Both boolean inputs in RelationCards now render CheckboxWidget, and the ~35 lines of local `.inline-edit-checkbox` CSS are deleted. Supersedes RR-CBLEV8 (which proposed extracting a shared stylesheet — using the existing widget is strictly better, since it shares behaviour and accessibility, not just paint). Verified in the running app: the form field and the RelationCards meta field now render identically.'
status: addressed
---

The second call site turned out to be a *third* variant, not a second: the "new
relation" form rendered a bare `<input type="checkbox">` with no class at all,
so it had never received any of the styling. Consolidating fixed a surface
nobody had reported.

**Why the widget rather than a shared stylesheet.** RR-CBLEV8 proposed
`src/styles/checkbox.css` with a `.rela-checkbox` class. Rendering the
component is better: a stylesheet shares only paint, while the widget also
carries the `forced-colors` fallback, the disabled/read-only distinction, and
the display-vs-edit arms. A future accessibility fix lands everywhere by
construction instead of by remembering to add a class.

**Behaviour preserved.** The card-level input keeps its ACL plumbing — the
`writable === false` verdict now travels through the widget's `disabled` prop
instead of a raw attribute, and is pinned by a test. The new-relation input
kept `v-model`; `newMeta` seeds properties to `''`, which the widget reads as
unchecked and replaces with a real boolean on first toggle, exactly as the
native input did.

**The guard is real.** RelationCards.test.ts had NO boolean coverage at all, so
the pre-existing 243 passing form tests proved nothing about this change. Four
tests were added and mutation-verified: reverting either call site to a raw
styled input fails all four. They assert the *widget* is mounted rather than
that a checkbox exists, which a re-introduced raw input would satisfy.
