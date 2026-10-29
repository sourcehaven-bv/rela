---
id: RR-7DCN75
type: review-response
title: Small simplifications
finding: decisionViews takes a messageType callback, but ct.message_types already holds the types (and the Keyed memo compares ct). The generation watcher that clears decidedGen is redundant, because generation only grows and the comparison already releases the lock (the channel-key watcher is enough). RlChannel's decisionsDisabled could be derived from the pending set (any 'decision:' key) plus submitting, instead of being one more prop threaded through RlChannel and RlChannelMessage.
severity: nit
resolution: decisionViews reads ct.message_types instead of a callback, the generation watcher is gone, and the disabled state derives from the pending set (with a decision:* key while the composer submits) instead of an extra prop.
status: addressed
---
