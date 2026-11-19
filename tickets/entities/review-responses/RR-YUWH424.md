---
id: RR-YUWH424
type: review-response
title: The shell frame can still be scrolled
finding: 'overflow: hidden keeps the frame a scroll container; any scrollIntoView or focus can shift it.'
severity: significant
resolution: 'The frame now uses overflow: clip; appShellFrame.test.ts pins it.'
status: addressed
---
