---
id: RR-GRAGS9
type: review-response
title: 'sqlite: due index in channelsDDL breaks Open on every existing v24 database'
finding: 'Approach 3 puts CREATE INDEX channels_session_due_idx ON channels(type, state, session_next_due_at) into channelsDDL. channelsDDL is part of schemaSQL (internal/sqlitedb/sqlitedb.go:391), and schemaSQL runs on every Open BEFORE the ladder (sqlitedb.go:147, then c.migrate at :150). On a v24 file the CREATE TABLE IF NOT EXISTS is a no-op, the index statement references a column rung 25 has not added yet, and Open fails with ''no such column: session_next_due_at''. The repo already has the pattern for this: mentionsDDL and searchDDL run after the ladder (sqlitedb.go:156-168) for exactly this reason. Plan change: keep the two columns in channelsDDL but move the index to a new post-ladder DDL constant (or create it only in the rung plus that post-ladder IF NOT EXISTS). AC 7 must open a real v24 database file through Open (not call addChannelSessionTimers directly), or it cannot catch this.'
severity: critical
resolution: 'Plan updated: due index moved out of channelsDDL into post-ladder channelTimersDDL (like mentionsDDL); rung 25 adds columns only; AC 7 opens a real v24 file through Open.'
status: addressed
---
