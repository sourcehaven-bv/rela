---
id: RR-LV6F7T
type: review-response
title: The list of channel operation names is duplicated
finding: 'channelFold.ts:67 (OPERATIONS) repeats [''edit'',''delete'',''remove'',''release''] from RlChannelMessage.vue:157, and the app has the same names as MessageOp (frontend/src/types/channels.ts:386). If a fifth operation is added in one place only, hasTypeAction treats it as a type action, so every message that offers it becomes kept and quiet types stop working for that reader. Fix: export one `CHANNEL_OPERATIONS` constant from channelFields.ts (or types.ts) and use it in both library files.'
severity: minor
resolution: Fixed. channelFields.ts exports CHANNEL_OPERATIONS; channelFold.ts and RlChannelMessage.vue use it. An app test pins it to MESSAGE_OPS from types/channels.ts, which is already pinned to the server.
status: addressed
---
