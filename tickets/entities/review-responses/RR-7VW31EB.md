---
id: RR-7VW31EB
type: review-response
title: Lost pin connection silently releases the schema lock
finding: The shared lock lives on one idle connection. A restart, pg_terminate_backend or idle_session_timeout releases it while the pools reconnect; an erase then drops the schema under a live tenant whose search_path falls back to public.
severity: significant
resolution: dropSchema refuses while public holds any table, so a stale tenant's queries fail instead of reaching other data; docs state the lock's limits and require the restart. Pinned by TestErase_LostLockFailsClosed and TestErase_RefusesWhenPublicHasTables. A per-connection AfterConnect lock is not possible for the neoq pool, which takes only a DSN.
status: addressed
---
