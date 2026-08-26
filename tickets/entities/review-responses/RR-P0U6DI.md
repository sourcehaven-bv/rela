---
id: RR-P0U6DI
type: review-response
title: 'useDelayedPending: a follow-up operation inherits no minimum and blinks'
finding: |-
    The minDuration guarantee was not a guarantee. The gate carried three coordinated pieces of mutable state (a `hidePending` boolean, a `minTimer` handle and `visible`), which admitted a configuration the four-state model never named: visible, minimum already elapsed, source still pending. `minTimer` is null there, so when a SECOND operation settled it took the `minTimer === null` branch and hid instantly — zero minimum enforced.

    Reproduced at the real PendingButton timings (500/400) and found to be worse than first reported: the intermediate settle hid the indicator OUTRIGHT, and the follow-up operation then re-paid the full 500ms delay. So a rapid double-save produced a visible flicker mid-sequence — precisely the defect this composable exists to prevent.

    The existing test 're-entering while visible does not extend the minimum indefinitely' should have caught this and did not: it used delay 0 and re-entered while the min timer was still live, so it never reached the state where the timer had fired but `visible` was still true.
severity: critical
resolution: |-
    Rebuilt the machine around a single `shownAt` timestamp plus one hide timer, so the bad state is unrepresentable: if we are visible, `shownAt` says when the period began and the remaining hold is derived rather than tracked. `hidePending` is gone.

    Two behavioural changes fall out. (1) Re-entry while visible restarts the period only when the previous one is ALREADY SPENT — keeping it otherwise, so a rapid sequence cannot extend the display indefinitely (the original comment's concern, which was correct). (2) The hide is deferred through a 0ms timer rather than fired inline, so a settle immediately followed by a new operation lets that operation cancel the hide and adopt the display instead of flickering.

    `flush: 'sync'` turns out to be load-bearing for this, and for a different reason than originally documented: verified by probe that Vue's default pre-flush coalesces true->false->true within one tick down to a single `true`, which would silently merge two operations into one display period and lose the second one's minimum. The comment now says that instead of the incorrect race-condition claim it previously made.

    Two regression tests added, both at production timings: a follow-up operation after the minimum has elapsed, and an operation settling mid-window holding only the REMAINING time. Two older tests that asserted synchronous hiding were updated — the one-tick deferral is now part of the contract.
status: addressed
---
