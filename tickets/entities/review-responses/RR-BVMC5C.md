---
id: RR-BVMC5C
type: review-response
title: Redundant title tooltip and falsy-success trap in commentBody
finding: Two minor issues. (a) `toDOM` set `title` unconditionally, so a fully-visible short comment got a tooltip repeating text already on screen, and an empty comment got `title=""`. (b) `commentBody` returns `''` for `<!---->` — a FALSY success — so a caller writing `if (!commentBody(x))` would misclassify an empty comment as a non-comment; the `?? null` on the return was also dead, since a matched group is always a string.
severity: minor
resolution: (a) `title` is now set only when the chip may actually clip — block comments, or labels over a `TITLE_THRESHOLD` of 60 chars. (b) Dropped the dead `?? null` and documented the falsy-success contract at the return, stating callers must test `=== null` rather than truthiness. Added a test asserting an empty comment is distinguishable from a non-comment.
status: addressed
---

The reviewer also noted an empty comment renders as a zero-width unclickable
atom. That is true but left as-is: `<!---->` carries no guidance, appears in no
template, and giving it a visible placeholder would mean inventing UI for
content that says nothing. It still round-trips correctly, which is the property
that matters.
