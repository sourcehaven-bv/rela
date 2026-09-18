---
id: RR-W4CXNN
type: review-response
title: Hover trigger for the link panel was scoped but not implemented
finding: 'Planning chose ''hover plus cursor'' for the link panel, and the docs plus four source comments described a pointer route. No hover handling was ever written: there is no handleDOMEvents, mouseover or pointerover anywhere in the milkdown directory, and the panel is driven solely by caret position via refreshDerivedState. The comments actively misled a reader into thinking a pointer path existed and was covered.'
severity: significant
resolution: Deferred rather than rushed in during review. Every false claim of hover has been removed from the docs and the source comments, so nothing now describes behaviour that does not exist. The keyboard and caret routes are complete and tested, which is what the accessibility requirement actually depends on; hover was additive discoverability only. The docs were also corrected to say the panel appears BELOW the link, which contradicted both the code and the commit that fixed the placement.
reason: 'Deferred during the review phase rather than rushed in. Hover is additive discoverability only: it is by definition not keyboard reachable, so it can never be the primary route to any action, and the accessibility requirement rests entirely on the caret and toolbar paths, which are complete and tested. Implementing it now would add debounce timing and dismissal precedence to a floating surface that already carries two triggers'' worth of interaction rules, at the end of a ticket whose critical findings were all in that same area. The harm in the original state was the documentation, not the absence: docs and four source comments described a pointer path that did not exist, which misleads the next reader into thinking it was built and covered. Every such claim has been removed, so nothing now documents behaviour the code lacks. Worth a follow-up ticket if hover discoverability is still wanted.'
status: deferred
---

**Finding (code review of the implementation).** Planning settled on "hover plus
cursor" for the link panel. The cursor half shipped; the hover half did not, but
the documentation and comments were written as though it had:

- `docs/data-entry.md` described the panel appearing on an existing link.
- `linkPanelPosition.ts`, `LinkTooltip.vue`, `EditorToolbar.vue` and
`MilkdownEditorLinks.test.ts` each referred to a pointer or hover route.
- The unit test file claimed "real hover is the e2e suite's job"; the e2e suite
tested no hover either.

A reader would reasonably conclude a pointer path existed and was covered.

**Resolution — deferred, and the lie removed.** Adding a hover trigger during
review would mean new debounce and dismissal logic on a surface that already has
two triggers' worth of precedence rules, for a path that is by definition not
keyboard reachable and therefore cannot be the primary route to anything.

What was wrong was the documentation, and that is fixed: every claim of hover is
gone from the docs and the source. The panel is described as what it is —
caret-driven — and the toolbar is documented as the complete pointer-free route
(link button retargets when the caret is in a link, remove-link button beside
it).

One related correction in the same pass: `docs/data-entry.md` said the panel
appears **above** the link. It appears below, which is what the code does and
what commit `00cd8b07` explicitly fixed.

Worth a follow-up ticket if hover discoverability is still wanted.
