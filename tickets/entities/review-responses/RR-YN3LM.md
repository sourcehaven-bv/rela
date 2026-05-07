---
id: RR-YN3LM
type: review-response
title: Atomicity claim is overstated — Phase 2 deletes are best-effort and Phase 1 has no true rollback
finding: |-
    Plan: 'Atomicity per request via WithTx, commit-or-rollback together.' Code reality (transaction.go:138, 156): Phase 2 (deletes) is best-effort, ignores errors. Phase 1 mid-flight failure: rollbackRenamed deletes successfully-renamed targets but doesn't restore prior content — on-disk state is strictly worse than before.

    If PATCH removes 5 relations and adds 3, Phase 1 succeeds but Phase 2 fails on relation #3 (file lock, fs error) → transaction returns success but data:[] semantics partially failed. Worse: Phase 1 fails on rename #5 of 8 → 4 successful renames have prior content destroyed.

    Fix: soften the atomicity claim. Add an honest scope note: 'WithTx commit is two-phase. Phase 1 (renames) is mostly atomic but mid-phase failure can lose prior file content. Phase 2 (deletes) is best-effort and silently ignores errors. New PATCH inherits these limits — no worse than existing per-relation endpoints. We do NOT introduce a write-ahead log in this ticket. Document in API reference; revisit when we add a transaction log.'

    Don't claim true atomicity.
severity: significant
resolution: 'Decision #10 + Scope (OUT): atomicity is two-phase, not absolute. Phase 1 mid-flight failure can lose prior file content; Phase 2 is best-effort. Documented honestly in API reference. Per user: ''guaranteed atomicity is overrated for local tool of this type.'' AC #20 tests Phase 1 commit-failure preserves graph integrity.'
status: addressed
---
