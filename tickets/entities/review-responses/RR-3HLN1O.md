---
id: RR-3HLN1O
type: review-response
title: Log live region announces bulk prepends and reloads
finding: 'RlChannel.vue:175. role=log is a polite live region with aria-relevant additions. Load older inserts up to a page of messages at the top, and a full reload after the after_rev cap (R13) replaces everything; screen readers read all of it. aria-busy is tied to loadingOlder, which the app will typically clear in the same tick the messages land, so it does not suppress anything. Fix: keep aria-busy true until after nextTick of a prepend or reload (track it internally in the messages watcher), or move announcements to a separate visually hidden live region that announces only appended messages.'
severity: minor
resolution: The log is no longer a live region and a separate visually hidden polite region announces only messages appended at the end.
status: addressed
---
