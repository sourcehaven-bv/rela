---
id: RR-NZ6TL5
type: review-response
title: 'Shutdown never closed the LISTEN connection'
finding: 'neoq postgres_backend.go Shutdown: listenerConn lives outside the pool and was never closed. An orphaned LISTEN session that is never read stops Postgres from trimming its notification queue; once full every pg_notify fails.'
severity: significant
resolution: 'Shutdown now closes listenerConn under listenerConnMu after canceling the contexts, and listenerManager closes a connection it obtains after cancellation. TestShutdownClosesListenerConnection asserts the LISTEN session disappears from pg_stat_activity; it fails on the previous fork commit.'
status: addressed
---
