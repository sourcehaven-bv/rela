---
id: RR-IZZROK
type: review-response
title: Keyboard activation desynced the checkbox from the document, losing edits
finding: 'The node view bound the toggle to `mousedown` only. A checkbox is focusable, and pressing Space fires `click` with NO preceding `mousedown`, so the browser ticked the input natively while the document kept the old value. `stopEvent` and `ignoreMutation` both suppressed the DOM change, so it was never mapped back. Measured on `- [ ] todo`: after `box.focus(); box.click()` the document still read `- [ ] todo`, `input.checked` was `true`, and the write-back guard reported `unchanged`. Any later edit repainted the view and the tick silently reverted. A keyboard user could tick several boxes, type, save, and lose all of them with no error - guardWriteBack cannot help, because as far as the document was concerned nothing happened. The control was also not operable by keyboard at all, a WCAG 2.1 SS2.1.1 Level A failure.'
severity: critical
resolution: 'Moved the toggle to a `click` listener, which fires for both pointer and keyboard activation; `mousedown` now only calls preventDefault to keep the caret out of the input. The click is deliberately NOT cancelled: preventDefault on a checkbox makes the browser restore the pre-click checkedness after the handler returns (the HTML spec''s legacy-canceled-activation behavior), which also undid what `update()` set during the dispatch and left a ticked document under an unticked box. A first attempt did cancel it and passed in jsdom while failing in Chromium, which is what drove adding real-browser coverage. Now the browser''s flip stands and the dispatch makes the document agree; `update()` stays authoritative on later redraws, with early returns for the case where no write is possible. Covered by a jsdom test asserting the DOCUMENT (not input.checked) after `box.click()`, and by an e2e test pressing Space in Chromium and asserting both halves. Verified non-vacuous: the e2e test fails against the old mousedown binding.'
status: addressed
---

## Why the tests missed it

Every unit test drove the toggle via `dispatchEvent(new
MouseEvent('mousedown'))`, which is not how a browser delivers a click and is
not how a keyboard delivers activation at all. The test helper now fires
`mousedown` then `click`, matching a real pointer sequence, and a separate test
uses bare `click()` for the keyboard path.

The deeper lesson is that jsdom disagreed with Chromium on
legacy-canceled-activation, so a jsdom-only suite could not have settled this.
The e2e pair exists for exactly that class of question.
