---
id: RR-66SEO82
type: review-response
title: Ledger digests outlive deleted rows and removed remotes
finding: 'PR guidance G-015: the per-remote ledger kept field digests of rows deleted locally (until the source changed) and of detached rows; the ledger of a remote removed from remotes.yaml was never deleted; no retention was defined.'
severity: significant
resolution: 'Run sweeps the ledger first: a linked or detached row whose local entity or relation is gone becomes dropped with its digests cleared (TestRun_SweepClearsDigestsOfDeletedRows). remotesync.Prune deletes the ledgers of remotes no longer in remotes.yaml; PullRemote and scheduler start call it (TestPrune_DeletesLedgersOfRemovedRemotes, TestPullRemote). Docs and the threat model state the retention.'
status: addressed
---
