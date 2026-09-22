---
id: RR-EOIEPJ
type: review-response
title: Caret landed inside the link after insertion, so typing extended it
finding: 'useLinkUI.submit''s collapsed-caret branch moved the caret past the inserted text and its comment promised typing would continue outside the link. It did not: a position on a mark''s trailing boundary inherits the marks of the preceding node, so the caret carried the link mark. Verified: inserting LINK at a caret then typing TAIL produced [LINKTAIL](url) — the user''s next words silently joined the link text.'
severity: significant
resolution: Added tr.setStoredMarks([]) after the selection is set. Moving the caret is not sufficient; clearing the stored marks is what actually detaches the next keystroke. Pinned by a unit test that inserts, types, and asserts the tail stays outside the link — mutation-verified by removing the call.
status: addressed
---

**Finding (code review).** The collapsed-caret insert branch in
`useLinkUI.submit` set the selection to `from + text.length` and its comment
said this put the caret "after it so typing continues outside the link rather
than extending it."

It did not. Verified against the real editor: inserting `LINK` at a caret in
`ab` yields `a[LINK](https://x.test/)b` with the caret at position 6 and
`marksAt: ["link"]`. Typing `TAIL` then gives `a[LINKTAIL](https://x.test/)b`.

ProseMirror resolves a position on a mark's trailing boundary as carrying the
marks of the node *before* it, so the caret is outside the text but inside the
link as far as the next keystroke is concerned.

The comment stating the exact behaviour the code failed to deliver is the worse
half of this: it tells a reviewer not to check.

**Resolution.** `tr.setStoredMarks([])` after `setSelection`. The comment now
says why that call is the load-bearing one rather than describing the outcome it
was supposed to produce.

Pinned by a unit test that inserts a link from a collapsed caret, asserts
`storedMarks` is empty, then types and asserts the result is `[LINK](...)` and
not `[LINKTAIL]`. Mutation-verified: removing `setStoredMarks` makes it fail.
