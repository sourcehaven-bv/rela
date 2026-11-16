---
id: RR-F1NDL57
type: review-response
title: Find opens search on a list page
finding: The Find selector looked for the box inside .entity-list, but the app shell paints the list header outside it, so Find on a list went to /search. Found in manual testing.
severity: minor
resolution: Find matches .search-box input directly. The test builds the header outside the list, as the shell does.
status: addressed
---
