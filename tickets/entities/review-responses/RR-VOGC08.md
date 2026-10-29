---
id: RR-VOGC08
type: review-response
title: choiceViews docstring states the open-channel rule backwards
finding: 'channelView.ts:469 says ''There are none while ... (hasNewer), unless the channel is open, or when the caller may not post''. The code returns none when the channel is NOT open. Fix: ''There are none while hasNewer, when the channel is not open, or when the caller may not post the answer type.'''
severity: nit
resolution: 'Fixed the choiceViews docstring: none while hasNewer, when the channel is not open, or when the caller may not post the answer type.'
status: addressed
---
