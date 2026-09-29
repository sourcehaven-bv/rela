---
id: BUG-HEIAVS
type: bug
title: sqlitestore compares RFC3339Nano timestamps as strings, which misorders values with trailing zeros
description: 'sqlitestore stores updated_at with time.RFC3339Nano, which trims trailing zeros, so the string comparisons in the version sweep (windows() and updated_at < ?) do not match chronological order. Example: ''...00.1234Z'' sorts after ''...00.123449999Z'' although it is earlier. Effect: a row can miss a sweep tick (one tick of delay in production; possible flakes in tests that sweep immediately after a write). Fix: a fixed-width format plus a migration for stored values. Found in review of BUG-07DNNY.'
priority: low
effort: s
why1: The sweep selects rows with updated_at < ? and orders by created_at as TEXT, and sqlitestore wrote those columns with time.RFC3339Nano.
why2: RFC3339Nano trims trailing zeros and drops a zero fraction, so values differ in width and '…05.1234Z' sorts after the later '…05.123449999Z'. A caller-supplied UpdatedAt was also written in its own offset rather than UTC.
why3: The sweep comment asserted RFC3339Nano is fixed-width and zero-padded. That is true of RFC3339's shape but not of Go's Nano layout, and no test checked string order against time order.
why4: The comments table had already hit and fixed the same flaw with a fixed-width layout, but noted the store columns as a separate concern and the fix stayed local.
why5: The on-disk format and the SQL that compares it lived in different files with nothing tying the ordering property to a test, so the assumption could be wrong without any check failing.
prevention: sqlitedb.TimeFormat is the one fixed-width UTC layout, written only through sqlitedb.FormatTime. TestFormatTimeSortsInTimeOrder pins string order equals time order (fails on the old format), and the v8 rung rewrites stored values (AM-sqlite-timestamp-order).
status: done
---
