---
id: RR-6D2GN6
type: review-response
title: Breadth.Tx shared maps across separate mutexes (data race)
finding: 'Breadth.Tx wrapped the transaction view in a fresh &Breadth{Store: view, ids: b.ids, calls: b.calls}. The child struct carries a zero-value sync.Mutex but shares the parent''s maps, so the child locked its OWN lock while writing the PARENT''s maps - synchronizing nothing. Concurrent access from parent and Tx view is unsynchronized map access: torn counts or the runtime''s fatal ''concurrent map writes''. The race detector runs in CI. This shape was copied verbatim from Counting.Tx (counting.go:181-186), which has the identical latent bug.'
severity: critical
resolution: 'The Tx view now delegates to the SAME Breadth rather than being a second one sharing its maps: a new unexported breadthView type holds a *Breadth parent and records through it, so both halves stay under the parent''s single mutex. Verified by mutation with an 8-goroutine concurrency test under -race: the old shape produces 8 DATA RACE reports, the new shape zero with exact counts (1200 ids / 800 calls). Counting was deliberately NOT changed - it is out of scope for a test-only ticket and has no concurrent call sites today - but breadth.go''s Tx doc now names the hazard so the shape is not copied back.'
status: addressed
---
