---
id: RR-K1XUFL
type: review-response
title: Lifecycle danger tone keyed on a magic op name
finding: 'RlChannel.vue:165. `op.name === ''delete''` hardcodes a realtime.yaml op name in a presentational component. Fix: add `danger?: boolean` to ChannelLifecycleDef (the app sets it), or a `dangerousLifecycle` prop.'
severity: nit
resolution: ChannelLifecycleDef gained danger, which sets the button tone, replacing the hardcoded delete name.
status: addressed
---
