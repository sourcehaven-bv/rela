---
id: RR-QEAALY
type: review-response
title: Missing play tests for the stateful composer paths
finding: 'RlChannelComposer.stories.ts covers post, required/max, every widget, reply and edit, but not the behaviour most likely to regress: switching type keeps each type''s draft; editing stashes and restores the new-message draft; reset() clears; a restricted ref in edit mode; an identity-only change of `editing`; refs across types. RlChannel.stories.ts has no test for follow-at-bottom or preserve-on-prepend, which the header comment promises. Fix: add one story per path. Most of the significant findings above would have been caught by them.'
severity: minor
resolution: Play tests now cover type switching, the edit stash and restore, reset, restricted refs, identity-only edit changes, per-type refs, follow-at-bottom and preserve-on-prepend.
status: addressed
---
