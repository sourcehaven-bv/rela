---
id: BUG-DQLE0K
type: bug
title: Channel cardinality column added by editing released migration 0026
description: 'PR #1899 added channel_types.cardinality by editing migration 0026, which shipped in v26.10.11 and v26.10.12. Databases that already ran 0026 never get the column, so enabling realtime.yaml stops the server at boot.'
priority: high
effort: s
status: wont-fix
---

## Description

PR #1899 (TKT-ZU7P13) added `cardinality TEXT NOT NULL` to `channel_types` by
editing `0026_channels.sql` and the SQLite channels DDL. 0026 shipped in
v26.10.11 and v26.10.12. The pg migrator tracks only the version number and
SQLite uses `CREATE TABLE IF NOT EXISTS`, so existing databases never get the
column. Once `realtime.yaml` is enabled, the type-record check fails and the
server does not start. Reported by a reviewer on #1899.

## Fix

Restore 0026 to its released content. Add pg migration 0027 and a SQLite schema
rung that add the column with an empty default. Treat an empty cardinality as
"recorded before cardinality was tracked": it matches any configured cardinality
and the next record fills it in.

## Resolution

Already fixed by #1902 (TKT-ZU7P13): it added pg migration 0029 and the SQLite
rung, and 0026 matches the v26.10.12 release again. Closed without a separate
change.
