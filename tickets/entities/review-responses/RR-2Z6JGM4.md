---
id: RR-2Z6JGM4
type: review-response
title: Failed append leaves stored files
finding: An append that placed files and then failed to write the log left files no message lists.
severity: minor
resolution: A deferred under-lock cleanup discards placed files unless the log or a job stash holds the seq; an append without files clears leftovers at its seq. Tested in TestAppendFilesWithoutLogRecord.
status: addressed
---
