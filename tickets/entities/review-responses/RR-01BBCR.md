---
id: RR-01BBCR
type: review-response
title: Failed first descriptor load is not retried on the same page
finding: 'ChannelsPanel calls useChannelSchema() once at setup (ChannelsPanel.vue:22), which calls ensureLoaded() once. The store promises a retry after RETRY_BASE_MS, but only when ensureLoaded is called again, and EntityDetail stays mounted across entity navigation. After one failed load the panel stays hidden until a `refresh` event or a full remount. Fix: call channelSchema.ensureLoaded() in an immediate watch on props.entityType and props.entityId in ChannelsPanel.'
severity: minor
resolution: ChannelsPanel calls channelSchema.ensureLoaded() whenever the entity type or address changes, so a failed first load is retried on navigation within the store's backoff.
status: addressed
---
