---
id: RR-O2LOG5
type: review-response
title: Descriptor reload resets an edit or reply in progress
finding: 'OpenChannel''s `target` computed (OpenChannel.vue:80-85) depends on the props.channelType object, and ChannelsSection''s `open` computed passes a new object after every descriptor change. A channelSchema reload (any `refresh` event, or checkNames after an unknown name) gives a new target object, so watch(target) at line 203 fires and drops `editing`, `replyTo`, the errors and the memo, although the channel did not change. A user editing a message loses the edit when someone saves data-entry.yaml. Fix: watch a string key (type, address, channel type name, name), as useChannel''s targetKey does.'
severity: minor
resolution: OpenChannel resets its session state on a string key of entity type, address, channel type name and channel name, so a descriptor reload no longer drops an edit or reply.
status: addressed
---
