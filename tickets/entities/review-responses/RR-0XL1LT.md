---
id: RR-0XL1LT
type: review-response
title: The loaded argument of useMessageLink is redundant
finding: 'frontend/src/components/entity/channels/useMessageLink.ts:18 and OpenChannel.vue:493-498: RlChannel renders under v-else-if="ch.head.value" (OpenChannel.vue:517-518), so a non-null channelView already means the head is held. The `() => ch.head.value !== null` argument adds a parameter and a watch source that cannot change the result. Drop it, or document a case where the view is mounted without a head.'
severity: nit
resolution: ADDRESSED dropped the loaded argument; the composable godoc says OpenChannel mounts the log only once it holds the head.
status: addressed
---
