---
id: RR-R2RHK
type: review-response
title: 'Deferred: lock around StoreVersion doesn''t cover the read-then-write compound op'
finding: 'cranky-code-reviewer #7: the lock wraps only the Put. Callers are expected to advance version, but the actual read-compare-advance logic lives in cryptofs.FS.ReadFile, not LocalState. Two processes racing still produces correct outcomes from each writer''s perspective.'
severity: significant
reason: Reviewer is correct that the lock doesn't close the compound-op race, but closing it requires moving the observed-vs-stored comparison into LocalState.AdvanceVersion (a new method). For this PR the lock is still useful as write-serialisation insurance — it ensures atomic rename, not interleaved writes. Tracked as a follow-up.
status: deferred
---
