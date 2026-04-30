---
id: RR-C9ZEF
type: review-response
title: SearchView removeFilter leaves stale URL params on empty query
finding: If user removes the last filter chip and the text query is empty, fullSearchQuery is empty, and search() early-returns. The URL still has stale ?q=... params. Pre-existing behavior, but the extraction means it is now in the trail for this PR.
severity: significant
resolution: Extracted syncUrlFromState() in SearchView and call it BEFORE the early-return for empty query. Removing the last filter chip (or clearing the text query) now clears stale URL params consistently.
status: addressed
---
