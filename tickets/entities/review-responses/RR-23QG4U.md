---
id: RR-23QG4U
type: review-response
title: useChannel keeps running after scope disposal
finding: useChannel.ts never clears `session` on unmount. A run() loop that is dirty, or a follow in flight, keeps issuing GETs and assigning refs after the component is gone. Add onScopeDispose(() => { session = null }) so current(s) fails and the loop stops. Same for useEntityChannels (bump seq).
severity: nit
resolution: useChannel clears its session and deadline timer in onScopeDispose and useEntityChannels bumps its sequence, each with a test.
status: addressed
---
