---
id: RR-FVMO2Y
type: review-response
title: Unsyncable local ids reach the wire (no client-side allowlist)
finding: Push keys come from the local working copy, which never passed the server's validIDSegment allowlist. An id with a path separator, '..', or control char would emit a doomed request returning an opaque 400 instead of failing fast locally.
severity: significant
resolution: Added syncableKey/validIDSegment mirroring the server allowlist; push now skips and reports unsyncable records (OutcomeInvalid) like it skips locked ones. Test TestPush_UnsyncableLocalID_SkippedAndReported covers it.
status: addressed
---
