---
id: RR-6VX1JC
type: review-response
title: 'resolve returns a raw producer value and said nothing about it'
finding: 'flattenToLine no longer has a single owner. The reader of applySteps has to trust a comment about a function defined 220 lines away. This is not a bug today, it is the setup for one: the next person adding a step type will either call payload.interpolate and inherit the guarantee without knowing it exists (fine), or call payload.resolve directly and silently bypass it (not fine). resolve is one method away at line 788, and its doc comment said only "looks up one {{...}} reference, returning \"\" when absent" — nothing about its return value being unsafe to use raw. Put the warning where the footgun is.'
severity: significant
resolution: 'Extended resolve''s doc comment to say it returns the RAW producer-supplied value, that this is not safe to write into a document as-is, and that callers should use interpolate — naming flattenToLine as the guard that calling resolve directly would bypass. Verified resolve currently has exactly one caller, the flattenToLine-wrapped substitution in interpolate, so no bypass exists today; the comment exists to keep it that way. The suggested rawValue named type was considered and not taken: it would touch the stringifyWebhookValue and lookupPath signatures for a single-caller unexported method, which is a larger change than this ticket''s scope and better justified on its own.'
status: addressed
---

The invariant is enforced at one call site and documented at another. The gap
between them is where the next bypass would be written.

The fix is one sentence at the point of misuse rather than a third paragraph in
`flattenToLine`'s comment.
