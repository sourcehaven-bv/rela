---
id: RR-OUNS3IW
type: review-response
title: Relation push after a failed merge overwrote the remote
finding: When the push-time refresh merged a relation and the local write failed (a version conflict or another error) the push still sent the local meta with If-Match and could overwrite the remote's newer value.
severity: significant
resolution: mergeRelation now returns whether the merge was written. refreshRelation stops the push when it was not. TestRelationPush_FailedMergeIsNotSent pins it.
status: addressed
---
