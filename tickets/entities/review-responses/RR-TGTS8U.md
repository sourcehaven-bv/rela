---
id: RR-TGTS8U
type: review-response
title: Nil entity from the iterator panics on the startup path
finding: The backfill loop dereferenced whatever the ListEntities iterator yielded. The contract is (nil, err) on failure, but nothing enforces it, and a (nil, nil) yield panics inside the indexer. This runs during openBackend — startup, with no recover — so it would kill the process, bypassing the deliberate graceful degradation where a broken index only logs a warning and leaves the store usable. Pre-existing, but the loop was being rewritten.
severity: significant
resolution: Added an explicit nil check that skips the entry, with a comment naming the contract it is defending against.
status: addressed
---
