---
id: RR-F9UB45
type: review-response
title: The story helper firstVisibleRow repeats channelAnchor.firstVisible
finding: 'RlChannel.stories.ts:603 reimplements the anchor scan (fold detection, measuring the button). Fix: import firstVisible from ./channelAnchor, so the story measures what the component measures.'
severity: nit
resolution: Fixed. The stories import firstVisible from ./channelAnchor; the story helper is gone.
status: addressed
---
