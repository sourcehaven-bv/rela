---
id: RR-NA7PYH
type: review-response
title: Channel wake drops a due comment poll
finding: A wake and the poll merge into one passWake, which skipped list_comments targets, so a steady stream of wakes could starve comment replays.
severity: significant
resolution: watchRun.commentsDue records a due comment replay apart from the pass kind; any pass but passInbox replays comments while it is set. TestWatchRun_CommentsDueSurvivesMergedPasses.
status: addressed
---
