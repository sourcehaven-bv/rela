---
id: RR-4SN00Y
type: review-response
title: aria-live region announces nothing (aria-hidden SVGs) + permanent idle live region
finding: The indicator's only content is aria-hidden SVGs, so the aria-live=polite region has no changing text to announce; the 'aria-live still announces' comment is false and the idle region is a permanent empty announcer.
severity: significant
resolution: 'Visual glyph wrapper is now aria-hidden=true; a separate visually-hidden .autosave-sr-only span (role=status, aria-live=polite) carries an `announcement` computed that is empty at idle and ''Saving…''/''Saved''/''Save failed'' otherwise. Live regions announce text content, which this now provides. Verified in real app: idle sr text is empty; an edit announces ''Saving…'' then ''Saved''. Misleading comment corrected. Added tests.'
status: addressed
---

**Finding:** The indicator's only content is `aria-hidden="true"` SVGs, so the
`aria-live="polite"` region has no changing text content to announce — the claim
"aria-live still announces state changes" is false. A permanently-mounted idle
live region also risks a spurious mount-time announcement and is SR-dependent.

**Fix:** Add a visually-hidden text node whose text changes with state (this is
what a live region actually announces), and gate/scope the live region so idle
doesn't sit as a permanent empty announcer. Correct the misleading comment.
