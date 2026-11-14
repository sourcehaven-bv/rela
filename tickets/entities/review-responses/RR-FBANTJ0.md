---
id: RR-FBANTJ0
type: review-response
title: An empty title map deletes nothing and requeues forever
finding: pgx sends a nil slice as NULL so the DELETE with type = ANY(NULL) kept every stale row and the types were requeued on every open.
severity: minor
resolution: The type list is a non-nil slice. TestSearchTitle_SyncQueue syncs an empty map and checks the table is empty.
status: addressed
---
