---
id: RR-SDPFDD
type: review-response
title: Gate wrote the drift ledger OUTSIDE the lock it took for the marker
finding: adoptWithDrift called adopt() (which acquired and released the lock around the marker write) and then performed the ledger read-modify-write unlocked. A gate adoption could therefore interleave its SaveLedger with a concurrent GC apply's ledger rewrite under the lock — last-writer-wins could resurrect consumed entries or lose a fresh FirstSeen, silently extending or resetting grace. The lock exists precisely to serialize ledger writers; one writer sat outside it.
severity: critical
resolution: 'Gate persistence restructured (commit d514cc8e): a single withLock() section now covers BOTH the marker write and the ledger record/prune/save; a contended gate skips the entire persist (verdict still published) so it can never interleave anything into a lock holder''s run. Pinned by TestGate_ContendedAdoptionSkipsLedgerToo (drift evaluation under a held lock: marker unmoved AND ledger empty).'
status: addressed
---
