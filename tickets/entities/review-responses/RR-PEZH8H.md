---
id: RR-PEZH8H
type: review-response
title: In-memory scorch runs no merger, so memory grows without bound
finding: 'The originally proposed fix swapped bleve.NewMemOnly for in-memory scorch, which halved startup RSS. But scorch.Open starts its persister and merger goroutines only when the index path is non-empty (scorch.go:311); with an empty path neither runs, so the segment produced by every write is never merged away. Measured on the real 2443-entity corpus, 1500 single-entity edits: 562MB when spread over 100 distinct docs, 1845MB over 500, 5711MB over the whole corpus — against a flat 17MB on the existing upsidedown engine. The change would have traded a bounded one-time startup spike for unbounded growth in exactly the long-lived MCP processes that motivated the ticket.'
severity: critical
resolution: In-memory scorch was abandoned as the primary fix. The index is now persisted on disk, which is what makes the persister and merger run (heap stayed at 9MB across 4500 edits). NewMem is retained only as a fallback for when the on-disk index is locked, and its godoc now documents the growth characteristic and points callers at New.
status: addressed
---
