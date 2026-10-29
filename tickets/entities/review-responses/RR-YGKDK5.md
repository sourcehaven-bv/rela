---
id: RR-YGKDK5
type: review-response
title: TKT-YK30D1 is pg_notify only and will not make the fs PoC live
finding: TKT-YK30D1 is "a pg_notify poke for channel changes into realtime.Local" (tickets/entities/tickets/TKT-YK30D1.md, backlog, low priority). The PoC runs on the default fs backend (rela-agent-runner-poc README.md:107-116), where channels live in the cache directory (internal/appbuild/channels.go:53) and no watcher reports them to rela-server. The plan's "What waits for TKT-YK30D1" section and the docs AC 12 imply that YK30D1 closes the gap for the PoC. They should say that cross-process session moves without graph outputs stay reload-only on fs and SQLite even after YK30D1, and Q5's MCP typing tool should be scoped the same way.
severity: minor
resolution: The "Cross-process limits" section, AC 12 and Q5 now say TKT-YK30D1 is postgres-only, and that on fs and SQLite a cross-process move without graph outputs stays reload-only.
status: addressed
---
