---
id: RR-U7VL4D
type: review-response
title: Channel state leaks across entity-to-entity navigation
finding: 'EntityDetail is reused when the route goes from /entity/feature/A to /entity/feature/B (EntityView renders EntityDetail without a key), and ChannelsPanel at frontend/src/components/entity/EntityDetail.vue:3112 carries no :key. ChannelsSection therefore keeps `selected`, `filter` and `creating` (ChannelsSection.vue:41-45, 90), and OpenChannel is keyed only by `open.key` = channelType/name (ChannelsSection.vue:146), which is the same for every entity with a `one` channel. OpenChannel''s watch(target) (OpenChannel.vue:203-208) clears replyTo, editing and errors but never calls composer.reset(), so RlChannelComposer keeps its per-type `drafts`; the `pending` set (keyed by seq) also survives. Result: a half-typed message written on entity A is still in the composer on entity B and Send posts it to B; a pending marker on seq N of A shows on seq N of B; a named channel selected on A is opened (and 404s) on B. Fix: key the panel by address in EntityDetail, `<ChannelsPanel :key="entityRef(entry)" ...>` (or key ChannelsSection by entityId and OpenChannel by entityId plus open.key), and add a ChannelsPanel test that changes entityId with a draft in the composer and asserts the draft and the selection are gone.'
severity: significant
resolution: ChannelsPanel keys ChannelsSection by entity type and address, so a draft, selection, edit or pending marker never carries over, and a ChannelsPanel test changes entityId with a draft and asserts it is gone.
status: addressed
---
