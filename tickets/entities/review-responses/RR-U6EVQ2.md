---
id: RR-U6EVQ2
type: review-response
title: tombstoneConfirmer keeps handles whose Restore failed
finding: realtime_worker.go:76 drop runs only after a successful Untombstone, so a failed restore leaves the handle in pending. A later successful delete of the same id then confirms that stale batch too.
severity: minor
resolution: Restore drops the handle before calling Untombstone, so a failed restore leaves no stale handle.
status: addressed
---
