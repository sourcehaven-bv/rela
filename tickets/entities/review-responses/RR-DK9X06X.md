---
id: RR-DK9X06X
type: review-response
title: Live hint never reaches a hidden tab
finding: useEvents wraps every live:changed frame in runWhenVisible, so the notifier's handler would only run when the tab is shown.
severity: critical
resolution: 'Plan adds a per-key deliverHidden option to registerLive/handleLiveChanged; only the elected notifier sets it. Unit test: hidden page runs it and still defers other keys.'
status: addressed
---
