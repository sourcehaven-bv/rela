---
id: RR-6JJCRQ
type: review-response
title: filechannels Untombstone comment claims the recompute corrects a crash before resume; it fires the timer instead
finding: 'ops.go Untombstone godoc (~line 684): ''A crash in between leaves the channel restored with its old schedule, which the timer loop''s recompute corrects.'' The recompute only derives NextDueAt from the stored anchors. It cannot add the missing restart anchor, so a timer that came due while tombstoned fires in the first round after the restore, not one period later. Either run resumeRestored before the batch directories are moved back (as sqlchannels.resumeBatch does, inside the step), or state the real outcome in the comment.'
severity: minor
resolution: filechannels Untombstone writes the resumed schedule inside ~detached so the move restores it with its anchors; TestUntombstone_ResumeBeforeMove injects failures; comment corrected.
status: addressed
---
