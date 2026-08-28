---
id: RR-WF0ZYF
type: review-response
title: Unbounded pg_advisory_lock at store-open can hang boot
finding: 'Reconcile takes the BLOCKING pg_advisory_lock, and appbuild calls it with context.Background() (no deadline). If a peer holds the reconcile lock (a long dry-run, or a peer mid-reconcile), pool.Acquire/the lock Exec blocks until release, stalling store-open. Two booting processes contending on this lock is the expected case given the multi-writer premise. FIX: use pg_try_advisory_lock with a bounded retry (or a ctx timeout), so a held lock degrades to the same non-fatal warning as any other reconcile failure rather than hanging boot.'
severity: significant
resolution: Reconcile now uses pg_try_advisory_lock (non-blocking). If a peer holds the lock, it returns store.ErrReconcileBusy (a non-fatal sentinel) rather than blocking boot on the background context. appbuild logs it at Debug and continues (specs still published so violations map); the CLI reports 'another reconcile in progress'. Reconcile is idempotent so the holder's pass covers this process.
status: addressed
---
