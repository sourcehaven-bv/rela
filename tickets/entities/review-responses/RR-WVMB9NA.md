---
id: RR-WVMB9NA
type: review-response
title: Focus-follow rule had gaps
finding: Focus on the jump button was not tracked, a header to row move would scroll the page, selectors were not scoped to one channel (two sections per page), and header focus uses preventScroll so it can land off screen.
severity: significant
resolution: 'Plan 5 and AC 7: jump button tracked; header to row goes to the jump button without scrolling; lookups scoped to the OpenChannel root; header focus after scrollIntoView nearest; tests for each case.'
status: addressed
---
