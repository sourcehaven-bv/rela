---
id: RR-G60CGD
type: review-response
title: channelNewMessages contract change breaks existing localisation overrides
finding: 'frontend/packages/rela-components/src/composables/useMessages.ts:311,407-414 and RlChannel.vue:216-232: `count` may now be 0 (a lone notice), and notices are passed in a new optional `notices` field. An app that overrides channelNewMessages through provideMessages under the old contract (count >= 1, no notices) now announces "0 new messages" for a lone notice and never reads any notice text. rela-components is a published library (README documents provideMessages). Fix: keep channelNewMessages unchanged for the non-notice part. Build the notice sentences in RlChannel and join them there, or add a separate channelNewNotices message, and skip channelNewMessages when count is 0.'
severity: minor
resolution: channelNewMessages keeps its old contract; a new channelNewNotices announces notices; two stories check an app override.
status: addressed
---
