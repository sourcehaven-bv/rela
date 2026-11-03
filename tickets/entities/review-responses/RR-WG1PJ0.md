---
id: RR-WG1PJ0
type: review-response
title: fs Append can report a committed post as failed when the reads.json write fails
finding: 'On the file tier the marker write comes after the log records, which is after the message is durable. If the plan returns that error, as the audit append does today, the caller sees a failed post. The SPA or an agent retries, and the message is stored twice. On SQL the upsert is inside the transaction, so this does not happen there. Fix: on filechannels, a failed reads.json write after the log records is logged with a warning (key, user, no content) and swallowed; Append returns the message. The marker is best effort there, as the crash window already accepts. Add a filechannels test that injects a write failure on reads.json: Append succeeds and the marker is unchanged.'
severity: significant
resolution: filechannels logs a failed reads.json write after the log records as a warning and swallows it; Append returns the message. TestAppend_MarkReadWriteFailure.
status: addressed
---
