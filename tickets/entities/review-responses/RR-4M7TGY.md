---
id: RR-4M7TGY
type: review-response
title: A decision in flight across a channel switch acts on the next channel
finding: 'OpenChannel is reused across channels (OpenChannel.vue:271-279 resets reply/edit/errors but not `deciding`). useSessionDecisions.ts:169-177 calls `d.ch.refreshHead()` and then `d.focusRow(cause)` after the POST settles. By then requireSession() (useChannel.ts:479) returns the new channel''s session, so the new channel''s head is fetched and a row with the old channel''s seq gets focus. In the refusal path (useSessionDecisions.ts:129-132), selectInComposer and setFormError write into the new channel''s composer. The new channel''s composer and decisions also stay blocked until the old request settles. Fix: record the channel key at click time and skip the after-steps (refresh, focus, form error) when it changed. Or let useChannel expose a session token that the composable compares.'
severity: minor
resolution: Each click records a token and the channel key, OpenChannel's channel-switch watcher calls resetDecisions() to release the lock and pending key at once, and the old click skips refresh, focus and form errors when either changed (two unit tests in useSessionDecisions.test.ts).
status: addressed
---
