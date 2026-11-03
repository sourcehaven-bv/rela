---
id: RR-AG4B89
type: review-response
title: Pre-v19 soft-deleted restore path untested
finding: Unmark rewrites search_text for a row soft-deleted before v19, but no test restores such a row and searches for it.
severity: minor
resolution: TestSearch_MigrationRebuildsIndex now soft-deletes OLD-3 before the upgrade, clears its search text, restores it and finds it by search. Removing the rewrite in Unmark fails the test.
status: addressed
---
