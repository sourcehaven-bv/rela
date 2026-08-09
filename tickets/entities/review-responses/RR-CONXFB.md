---
id: RR-CONXFB
type: review-response
title: The load-bearing CSS rule has no test; my claim that jsdom couldn't cover it was wrong
finding: |-
    I recorded in the review checklist that three layout bugs were found by eyeball rather than tests, and implied the gap was unavoidable because jsdom can't do layout.

    That premise is false. The reviewer tested it: jsdom loads the real properties-list.css, resolves the `var(--field-span, 12)` fallback, and applies the .property-long override correctly --

      display: grid   align-items: start
      unspanned:    span 12   (fallback works)
      spanned:      span 4    (--field-span: 4)
      longWithSpan: span 12   (.property-long beats authored span)

    What jsdom genuinely cannot do is box geometry -- track sizing, wrapping, actual pixel widths. But NONE of the three bugs needed geometry. All three were cascade-resolution failures: the equal-specificity collision that swallowed every authored span, the relation widgets becoming auto-width grid items, and the align-items stretch. jsdom resolves cascade.

    So a ~20-line test that injects the stylesheet and asserts getComputedStyle(el).gridColumn would have caught two of the three, and an align-items assertion the third.

    This matters more than any individual bug: the entire design hinges on one CSS declaration in one file that no test touches. fieldSpan.test.ts tests the helper that PRODUCES the custom property, not the rule that CONSUMES it. Three real bugs shipped through that gap.
severity: significant
resolution: |-
    Fixed in 2ff8e0db. Added frontend/src/styles/propertiesListGrid.test.ts: it injects the real properties-list.css into jsdom and asserts computed style for six behaviours -- the grid and align-items:start, the var() fallback giving span 12, authored spans applying, .property-long overriding an authored span, the compact side-panel variant collapsing to one column, and labels not being uppercase.

    Critically, I mutation-tested it rather than trusting that it passes. Removing `align-items: start` fails a test. Replacing `span var(--field-span, 12)` with a hardcoded `span 12` fails a test. Both mutations reproduce bugs that actually shipped in this PR, so the tests kill real defects rather than passing vacuously.

    The review was right and my earlier claim was wrong: I had recorded that the eyeball-only gap was unavoidable because jsdom can't do layout. jsdom can't do GEOMETRY -- track sizing, wrapping, pixel widths -- but all three bugs were cascade-resolution failures, and jsdom resolves the cascade including custom-property fallbacks. The file's header says exactly that, and warns against adding geometry assertions that would pass vacuously.
status: addressed
---
