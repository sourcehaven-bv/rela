---
id: RR-UVW8YL
type: review-response
title: isCached and loading were not fenced with the content they describe
finding: isCached was assigned outside the content guard, so the cached badge could describe content other than what is on screen. loading was cleared unconditionally in finally, so a superseded render settling first re-enabled the Refresh button while the render the user was waiting on was still in flight.
severity: significant
resolution: isCached moved inside the generation fence beside the content assignment. The finally block clears loading only when the settling render is still the newest. Both applied to DocumentView and DocumentsPanel; the loading half is pinned by a test asserting the Refresh button stays aria-disabled when a superseded render settles first.
status: addressed
---
