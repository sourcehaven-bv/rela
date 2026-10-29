---
id: RR-NS5Z2P
type: review-response
title: onHeld defaults to a silent no-op
finding: 'useChannelPaging.ts:22 gives `onHeld` a default of () => {}, and the only caller (OpenChannel.vue:481) always passes it. A future caller that forgets it gets a jump that loads pages and then never shows the target, with no error. That is the silent no-op substitution that CLAUDE.md (''Constructors reject nil required fields'') warns against. Fix: make onHeld a required parameter.'
severity: minor
resolution: Fixed. onHeld is a required parameter of useChannelPaging.
status: addressed
---
