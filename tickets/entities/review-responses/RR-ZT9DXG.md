---
id: RR-ZT9DXG
type: review-response
title: Ambient timings ignore the 800ms autosave debounce, so the 0ms-delay rule is measured from the wrong instant
finding: |-
    The ticket specifies the ambient (autosave) class as "delay 0ms, min 600ms", and justifies the 0ms as "show immediately so the user perceives a smooth idle -> saving -> saved transition".

    But useAutoSave.ts:144-148 debounces before the request is even issued: baseDebounceMs = 800 (fieldDebounceMs/contentDebounceMs default to it), plus dirtyWindowMs = 1500. So the ambient indicator's "0ms delay" is measured from the START OF THE REQUEST, which is already at least 800ms after the user stopped typing.

    This matters for two reasons:

    1. The stated rationale is wrong. The user does not perceive idle -> saving -> saved as immediate feedback on their typing; they perceive a ~800ms gap, then a 600ms-minimum indicator. The design rationale in the ticket should say what is actually true, or the numbers should change.

    2. It makes the ambient class inconsistent with the governing rule in a way the ticket does not acknowledge. Every other class hides sub-delay work; the ambient class shows ALL work, because by the time it starts, the debounce has already filtered out the fast/transient cases. That is defensible — the debounce IS the ambient class's delay — but it should be stated, otherwise a future reader "fixes" the inconsistency by adding a delay and makes autosave feel broken.

    Recommend: document the debounce as the ambient class's effective delay, and cross-check that 600ms min is still right given the request only starts after 800ms of quiet.
severity: significant
resolution: 'Plan Approach §6 added. The plan now states that the ambient class''s 0ms delay is measured from REQUEST START, which useAutoSave.ts:144-148 has already deferred by baseDebounceMs = 800 — so the debounce IS the ambient class''s effective delay, and that is why no additional entry delay is needed. This is recorded as a required frontend/CLAUDE.md note precisely so a future reader does not ''fix'' the apparent inconsistency with the governing rule by adding a delay and make autosave feel broken. The ticket''s original rationale (''so the user perceives a smooth idle -> saving -> saved transition'') is corrected in the plan: the user perceives ~800ms of quiet, then the indicator. Re-checking whether 600ms is still the right minimum given the 800ms lead-in is called out as an implementation-time task.'
status: addressed
---
