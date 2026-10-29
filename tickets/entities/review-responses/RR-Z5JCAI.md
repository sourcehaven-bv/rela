---
id: RR-Z5JCAI
type: review-response
title: Composer hint can claim a guard where the caller has an unguarded edge, and names targets the caller cannot reach
finding: 'channelView.ts:330-346. toLabels removes duplicate targets and guarded is any(entry.guarded), so [{to: X, roles: [lead]}, {to: X, when: ...}] gives a lead "moves the session to X if its condition holds", although the lead''s entry has no condition. Role-split targets are shown to every caller. The plan accepts this (Q5), but the single-target guarded wording is plainly wrong, not only imprecise. Set guarded only when every entry of the merged target is guarded, or use the branch "may move" wording whenever the merged entries differ in guard.'
severity: minor
resolution: composerTransitions sets guarded only when every entry is guarded, and sets a new mayMove flag when only some are; the composer then uses the "may move" hint (channelView tests and the MixedGuardTransitionHint story).
status: addressed
---
