---
id: RR-CTX3
type: review-response
title: Race test did not exercise the race it claimed to guard
severity: significant
status: addressed
finding: raceStore injected the concurrent edge from INSIDE the collect, so the manager saw it, authorized
  it and denied. The post-collect TOCTOU window was never entered, and the test passed identically against
  a build with no Tx at all.
resolution: 'Rewritten to signal after both collects complete. Fixing it exposed two further bugs in the
  test itself: the Tx override passed the outer decorator, so tx.DeleteEntity re-entered MemStore.DeleteEntity
  and self-deadlocked on an already-held txMu; and the assertion checked for a dangling relation that
  memstore never prevents, since endpoint validation lives in entitymanager. The corrected test fails
  against the naive no-Tx design, verified by mutation.'
---
