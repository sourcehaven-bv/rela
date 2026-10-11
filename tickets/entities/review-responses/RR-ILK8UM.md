---
id: RR-ILK8UM
type: review-response
title: A failed comment count failed the whole list with a 500
finding: One unreadable thread file or a database error made GET list?comment_counts=true return 500, so the kanban board did not load.
severity: significant
resolution: Fixed. A failed Count is logged with slog.WarnContext and the rows are served without _comment_count. Pinned by the 'count failed' subtest in TestCommentCounts_AbsentWhenNotServed.
status: addressed
---
