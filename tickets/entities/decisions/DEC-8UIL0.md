---
id: DEC-8UIL0
type: decision
title: 'Write serialization becomes a store transaction contract: Tx on store.Store (fs = mutex, postgres = native transactions)'
context: |-
    dataentry.App.writeMu is a process-local mutex serializing only the data-entry HTTP mutation handlers — evolutionary residue from rela's markdown-on-disk origins, now running the postgres backend under the filesystem backend's concurrency model. Research RES-Z1SJ5 (2026-07-16 codebase survey) established: entitymanager intents are already multi-write and non-atomic (create = up to 2 store writes + cascade; delete = version-capture-then-delete, with manager.go:689 calling strict atomicity 'a future hardening'); CLI/MCP/scheduler write via entitymanager with no serialization at all; postgres has no cross-process serialization of ordinary writes; pgstore already wraps every write in a transaction (for pg_notify atomicity) and fans out in-process events post-commit. Five options were evaluated against five concrete anomaly classes (check-then-act races, lost updates, partial intents, interleaved cascades, interleaved scripts). Option B — a per-backend transaction contract on the store — was selected: it is the only option fixing cross-process races AND intent atomicity, and it dissolves writeMu rather than relocating it.

    This decision amends the root CLAUDE.md rule 'No repository or transaction abstractions — the old repo and tx layers are gone, do not reintroduce equivalents.' The removed layers were generic indirection stacked ABOVE the store; this is a contract ON store.Store itself with per-backend meaning. The rule text should be updated to state the distinction explicitly.
consequences: |-
    ## Interface

    `store.Store` grows one method (an interface member, NOT an optional type-asserted capability — every in-tree backend can implement it, and entitymanager requires it for correctness, so 'optional' would force a dual code path through the most safety-critical package):

    ```go
    Tx(ctx context.Context, fn func(Store) error) error
    ```

    Contract v1: writes inside fn are isolated from every other transaction — cross-process where the backend can see them; observer/subscriber events are delivered post-commit; errors roll back where the backend supports it. Enforced by a new storetest conformance section (including re-entrancy and post-commit event assertions). The contract specifies BEHAVIOR, not mechanism — each backend is free to change its serialization implementation without touching callers.

    ## Per backend

    - **fsstore/memstore**: one plain write mutex held across fn. Go deliberately has no reentrant mutex, so effective re-entrancy comes from structure: public write methods become thin Lock-then-unlocked-core wrappers, and Tx locks once and hands fn a view bound to the unlocked cores. Isolation only — no rollback (error mid-fn leaves partials, same as today) and no crash atomicity (no WAL; don't pretend). Events buffered, emitted after unlock.
      - **Considered alternative: single writer goroutine (queue/actor).** Equivalent serialization, but it does not remove the need for reader synchronization — fsstore readers RLock the live maps, so a queue would ADD a mechanism next to the RWMutex rather than replace it. The variant that genuinely pays — single writer goroutine + copy-on-write snapshots published via atomic.Pointer (readers lock-free; the repo's own publish/serialize split rule) — is a real fsstore state-model rearchitecture, noted as fsstore's natural v2, out of scope for the Tx arc. The queue also adds ctx-cancellation-while-queued, panic-forwarding, and shutdown-drain semantics. Mutex chosen for v1 as the smallest conforming implementation.
    - **pgstore**: BEGIN + one global pg_advisory_xact_lock(write key) + a tx-bound store view over the existing DBTX seam (pgx.Tx already satisfies it). Full rollback on error. pg_notify already fires at commit; in-process per-write events are buffered until Tx commit. Global key = zero deadlock risk and no retry machinery; per-entity lock granularity is a later optimization inside pgstore, invisible to callers.

    ## Boundary and nesting

    entitymanager wraps each intent (create incl. automation re-write, update, delete incl. version capture, rename, relation ops) in one Tx; cascadeHost and the version writers receive the tx-bound store, so cascade side-effects join the intent's transaction. Nesting = joining: there is no nested-Tx API; an intent opens a Tx only when not already inside one.

    **Open sub-decision for the design review — how joining is detected:**

    1. **Explicit tx-bound store (chosen, v1 default):** membership = holding the store view Tx handed to fn. Visible in every signature; costs threading the view through entitymanager's ~11 write sites + cascadeHost + version writers. Misuse (sharing the view across goroutines spawned inside fn) is possible but explicit.
    2. **Implicit ctx-carried join (documented fallback):** Tx stamps the ctx; public write methods detect the marker and join the running transaction inline. Works identically under mutex or queue (an executor-identity check is the actor-runtime spelling of the same idea, but Go deliberately exposes no goroutine identity, so ctx is the honest carrier). Enormous churn reduction — no store threading at all — and consistent with the existing Principal/version-attribution-via-ctx precedent. Rejected as v1 default because it makes the write path's most load-bearing invariant invisible in signatures (behavior-switching via ctx), and its failure mode is SILENT loss of mutual exclusion if fn's ctx leaks into a spawned goroutine (explicit-view and identity-based variants fail loud with a deadlock instead). If entitymanager threading proves substantially uglier than estimated, revisit — the two schemes are behaviorally identical for correct code and switchable without contract changes.

    Audit records are emitted post-commit (same 'after the durable write' guarantee, correctly generalized — an audit row must never exist for a rolled-back write; denied-write audit rows, emitted before any write, are unaffected).

    ## Lua

    Write-path scripts get a closure-based grouping helper — `rela.tx(function() ... end)` — which runs its closure's Mutator intents inside one store Tx via a manager-level grouping API (Manager exposing InTx(ctx, func(Mutator) error) or equivalent; same joining mechanism the intents use internally). An error inside the closure rolls back the group and re-raises in Lua. Hard rule enforced at runtime: `ai.*` bindings raise an error inside a transaction — no external I/O while holding the deployment-wide write lock. Whole scripts are never implicitly wrapped; scripts needing atomicity opt in with the helper. Consequence: dataentry's whole-script critical section is not preserved implicitly — writeMu is deleted outright, with rela.tx as the explicit replacement for scripts that relied on it. This is a documented behavioral change (cross-process on postgres, scripts already interleaved today).

    ## Rollout

    Sequenced as its own arc after TKT-R68TV8 M5.4 lands conservatively (M5.4 moves the mutex with the write nucleus, semantics untouched). Implementation order: storetest Tx contract → fsstore/memstore → pgstore → entitymanager wraps intents → Lua helper → delete writeMu. Gated on the race-detector suite and storetest conformance.

    ## Out of scope (v1)

    fs pre-image rollback journal; optimistic concurrency / revision CAS (lost updates remain — deliberately deferred, layers on top of this later); per-entity advisory-lock granularity; any retry-on-conflict machinery; fsstore single-writer-goroutine + COW snapshot state model (noted above as the natural fsstore v2).
date: "2026-07-17"
status: accepted
---
