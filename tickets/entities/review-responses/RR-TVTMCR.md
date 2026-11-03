---
id: RR-TVTMCR
type: review-response
title: Rendered-HTML memo grows without bound
finding: 'The MarkdownMemo at OpenChannel.vue:99 is replaced only on a target change. Every edit adds a new seq:rev:prop:world entry, and messages dropped by the 500 cap keep theirs, so a long-lived page on a busy channel keeps the HTML of every revision. Fix: after computing the views, prune keys whose seq:rev is no longer in ch.messages, or bound the map as an LRU.'
severity: minor
resolution: messageViews prunes memo entries whose seq and rev are no longer held, so the memo holds at most one entry per held message prop, pinned by a channelView test.
status: addressed
---
