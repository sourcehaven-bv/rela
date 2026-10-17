---
id: BUG-DQLE0K
type: bug
title: Channel cardinality column added by editing released migration 0026
description: 'PR #1899 added channel_types.cardinality by editing migration 0026, which shipped in v26.10.11 and v26.10.12. Databases that already ran 0026 never get the column, so enabling realtime.yaml stops the server at boot. The released file store wrote _types/anchor-modes.yaml, and #1899 switched to _types/records.yaml without reading the old file, so recorded anchor modes were lost.'
priority: high
effort: s
status: backlog
---

## Description

PR #1899 (TKT-ZU7P13) added `cardinality TEXT NOT NULL` to `channel_types` by
editing `0026_channels.sql` and the SQLite channels DDL. 0026 shipped in
v26.10.11 and v26.10.12. The pg migrator tracks only the version number and
SQLite uses `CREATE TABLE IF NOT EXISTS`, so existing databases never get the
column. Once `realtime.yaml` is enabled, the type-record check fails and the
server does not start. Reported by a reviewer on #1899.

The file store has the same problem in another form. The released file store
wrote each channel type's anchor mode to `_types/anchor-modes.yaml`. #1899
switched to `_types/records.yaml` and does not read the old file. A project
upgraded from a release therefore loses its recorded anchor modes, and the boot
check no longer refuses a changed anchor for channels that already exist.

## Fix

Restore 0026 to its released content. Add pg migration 0029 and a SQLite schema
rung (v20 to v21) that add the column with an empty default. Treat an empty
cardinality as "recorded before cardinality was tracked": it matches any
configured cardinality and the next record fills it in. The file store reads
`_types/anchor-modes.yaml` as records with an unknown cardinality. A test pins
the content of every released pg migration.
