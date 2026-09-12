---
id: RR-2QT4XY
type: review-response
title: Toast pointer-events opt-back-in was scoped too widely and still swallowed clicks
finding: 'The first fix put pointer-events:none on .toast-container and auto back on .toast. That was too coarse: the toast''s own icon and message spans inherit auto from .toast, so they kept intercepting clicks aimed at what sits underneath. Because the action keeps the user on the form, what sits underneath is the very button they need for the next record. Surfaced as intermittent ''toast-message intercepts pointer events'' failures under parallel e2e load.'
severity: significant
resolution: Narrowed the opt-back-in from .toast to .toast-dismiss, so only the dismiss control takes pointer events. Verified the dismiss button still works (dedicated e2e check) and confirmed stability with two consecutive full e2e runs (285 passed, 0 failed).
status: addressed
---
