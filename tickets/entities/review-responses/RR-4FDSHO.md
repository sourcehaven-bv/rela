---
id: RR-4FDSHO
type: review-response
title: Migration-lock review minor/nit batch (7 findings)
finding: 'Batch: (5) LockFor doc claimed memory backend gets a process lock while the cacheDir branch wins; (7) ctx cancellation during a sweep-tick lock acquire logged as a scary warning at shutdown; (8) CLI built a fresh lock value per call so the fs in-process mutex spanned nothing, and appbuild/CLI were asymmetric; (9) a stale release func from an earlier acquisition could unlock a later ProcessLock holder; (10) fsLock returned a bare unwrapped ctx error; (11) MkdirAll inside the retry loop; (12) guide overstated the gate guarantee (ledger) and lacked the stale-lock caveat.'
severity: minor
resolution: 'All addressed (commits 3222c401/d514cc8e): (5) comment rewritten — the fs lock for memstore-with-project is CORRECT and now explained (the guarded marker/ledger live in .rela/ FSKV for every non-postgres backend, so the lock must be a file beside them); (7) context.Canceled ticks skipped silently in the sweep loop; (8) each CLI command now builds ONE lock value threaded through gate/runner/GC, matching appbuild''s one-lock-per-store; (9) ProcessLock releases are generation-guarded; (10) wrapped with the package prefix; (11) hoisted; (12) guide updated — ledger claim now true after the RR-SDPFDD fix, and the pid-reuse caveat + manual remedy added.'
status: addressed
---
