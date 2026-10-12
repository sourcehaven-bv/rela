---
id: RR-VQNYT3
type: review-response
title: Guard undo runs on the request context
finding: undoFailedDelete and the purge undo call Untombstone with the request ctx (anchor_guard.go:203, manager.go:1799/2256, softdelete.go:513). A client disconnect aborts the Tx with context canceled, the undo fails on the same ctx, and the live entity's channels stay tombstoned with the handle lost.
severity: significant
status: open
---
