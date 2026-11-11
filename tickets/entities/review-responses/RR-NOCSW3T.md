---
id: RR-NOCSW3T
type: review-response
title: filechannels Remove order leaves bytes after a crash
finding: mutateThen writes meta.yaml then the segment; deleting bytes afterwards can leave bytes of a removed message forever. SQL Remove is an UPDATE so no cascade fires.
severity: significant
resolution: File backend moves files/<seq> to trash and deletes it before the segment rewrite (fail closed). SQL Remove deletes channel_files rows for the seq in the same transaction.
status: addressed
---
