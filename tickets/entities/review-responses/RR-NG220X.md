---
id: RR-NG220X
type: review-response
title: isFresh could never return true, arming a fresh-install break at the next schema bump
finding: 'Conn.init ran schemaSQL (CREATE TABLE IF NOT EXISTS throughout) and only then called migrate, so by the time isFresh queried sqlite_master for `entities` the table always existed. The fresh branch was unreachable. Masked today because a fresh database falls through to found=1 and the single v1-to-v2 step is IF NOT EXISTS, so replaying it is a harmless no-op. At schemaVersion 3 it breaks: a brand-new database, already at the v3 shape from schemaSQL, replays the entire ladder from v1, and the first step written as an ordinary ALTER TABLE fails. Every new install would refuse to open, and the person who wrote migration 3 would have done nothing wrong.'
severity: critical
resolution: 'Freshness is now measured in Conn.init BEFORE schemaSQL runs and passed into migrate(ctx, fresh); isFresh returns an error rather than swallowing an unreadable sqlite_master, since assuming not-fresh replays the ladder. Verified the original claim by instrumenting isFresh (false on a brand-new database), then simulated migration 3 with a plain ALTER TABLE step: the pre-fix code failed with `duplicate column name: mode` on a fresh open, and TestFreshDatabaseSkipsTheLadder catches it.'
status: addressed
---
