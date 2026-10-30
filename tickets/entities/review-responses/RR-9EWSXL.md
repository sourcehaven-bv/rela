---
id: RR-9EWSXL
type: review-response
title: A "first page loaded" trigger does not guarantee the RlChannel ref is mounted
finding: 'OpenChannel calls channelView.value?.reveal(seq) (OpenChannel.vue:484). If channelView is not mounted yet, that returns undefined, not false. When goTo then runs onJump on a held message, it calls onHeld after one nextTick, which can still find no ref, and the reveal is silently lost. RlChannel''s first scroll to the bottom can also run after reveal and undo its scroll. Fix: watch the ref as well as the data, treat undefined as "not ready" rather than "not held", and add a deep-link test that mounts with the target already on the first page.'
severity: minor
resolution: ADDRESSED OpenChannel watches revealSeq, first page loaded and the channelView ref, treats a null ref as not ready, awaits one tick after mount so the mount scroll runs first; first-page deep-link case added.
status: addressed
---
