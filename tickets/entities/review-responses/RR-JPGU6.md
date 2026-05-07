---
id: RR-JPGU6
type: review-response
title: beforeunload guard suppression keys off lagging status, allowing pending changes to be lost on navigate-away
finding: |-
    Plan: 'Suppress beforeunload/route-guard warnings unless status === saving|error.' Sequence: user types 'abc' → debounce timer queued → status is still 'idle' or 'saved' → user navigates away → suppression kicks in → debounce timer fires after unmount → form is gone, PATCH never sent, data lost.

    Fix: before deciding whether to warn, call commitImmediately() synchronously to flush any queued debounce timer into an actual PATCH. Then warn iff there is at least one in-flight PATCH. Do not key the warning off status — key it off pending-flush + in-flight count, both managed in useAutoSave.
severity: significant
resolution: 'beforeunload and onBeforeRouteLeave handlers synchronously call commitImmediately() to flush queued debounce timers into PATCHes. Then warn iff useAutoSave.inFlightCount > 0 after flush — keyed off the queue state, not status. AC #12 covers both Vitest and Playwright.'
status: addressed
---
