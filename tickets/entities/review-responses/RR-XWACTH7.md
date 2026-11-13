---
id: RR-XWACTH7
type: review-response
title: Reply excerpts render markdown on every recompute
finding: excerptOf ran Marked, DOMPurify and DOMParser for each held reply on every messageViews pass, outside the MarkdownMemo.
severity: significant
resolution: excerptOf takes the memo and holds the excerpt by seq:rev; pruneMemo drops it with the revision. Unit test pins the memo.
status: addressed
---
