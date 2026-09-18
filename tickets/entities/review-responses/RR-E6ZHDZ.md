---
id: RR-E6ZHDZ
type: review-response
title: dom.test.ts did not pin checkbox/radio behaviour
finding: |-
    `isInputFocused()` returns true for `<input type="checkbox">` and `type="radio"` because the tag is INPUT, so `/` and `f` do not fire while a checkbox has focus. That behaviour was undocumented and unpinned.

    The risk is a plausible future 'fix': someone notices a checkbox accepts no text and narrows the guard to text-like inputs only. The same reasoning excludes contenteditable, which is exactly how BUG-DNP5E7 would return.
severity: minor
resolution: 'Added an `it.each([''checkbox'',''radio''])` case to dom.test.ts asserting the guard returns true, with a comment stating why the behaviour is deliberate: blocking a shortcut on a focused checkbox costs nothing, while narrowing the rule costs the bug.'
status: addressed
---

Finding 5 from the cranky-code-reviewer.
