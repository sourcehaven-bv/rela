---
id: RR-FB6QU8
type: review-response
title: Advisory lock must be held on the SAME connection that runs the sweep queries (else single-writer guarantee is void)
finding: 'pg_try_advisory_lock is SESSION-scoped — it gates only the connection that acquired it. The plan says the sweep runs on its ''own connection (like the listener)'' but if it acquires the lock on a dedicated connection and then issues its SELECT/INSERT via the pool (s.db), those run on DIFFERENT sessions that don''t hold the lock. Two processes could each hold the lock on their own dedicated connection yet both INSERT concurrently through their pools — the single-writer guarantee (AC-9, R1), the entire justification for the sweep redesign, silently evaporates. Passes a single-process test, fails in production. Fix: run the ENTIRE tick (pg_try_advisory_lock → select → all inserts → pg_advisory_unlock) on ONE acquired connection (pool.Acquire() → use that *pgxpool.Conn for every statement, or the dedicated raw conn). Make this an explicit design invariant + a test. Connection-death releasing the lock mid-sweep is safe/idempotent (full-snapshot + content-hash dedup).'
severity: significant
resolution: 'Fixed: the entire sweep tick (pg_try_advisory_lock → select → all inserts → pg_advisory_unlock) runs on ONE acquired connection (pool.Acquire() → the *pgxpool.Conn used for every statement). Documented as an explicit design invariant (R9) in the postgres CLAUDE.md; AC-10 is a two-connection integration test asserting no dup/lost rows and that the lock and queries share a connection.'
status: addressed
---
