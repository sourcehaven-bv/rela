---
id: RR-KQ444A
type: review-response
title: Reply control check on persist is dead code
finding: RlChannelMessage.vue:101-106 and 247. `messageType?.persist !== 'ephemeral'` can never be false for a row in the log, since ephemeral messages never enter it (types.ts:56). Drop it, or document why it stays.
severity: nit
resolution: The dead persist check on the reply control was removed.
status: addressed
---
