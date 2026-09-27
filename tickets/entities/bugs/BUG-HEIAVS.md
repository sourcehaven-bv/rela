---
id: BUG-HEIAVS
type: bug
title: sqlitestore compares RFC3339Nano timestamps as strings, which misorders values with trailing zeros
description: 'sqlitestore stores updated_at with time.RFC3339Nano, which trims trailing zeros, so the string comparisons in the version sweep (windows() and updated_at < ?) do not match chronological order. Example: ''...00.1234Z'' sorts after ''...00.123449999Z'' although it is earlier. Effect: a row can miss a sweep tick (one tick of delay in production; possible flakes in tests that sweep immediately after a write). Fix: a fixed-width format plus a migration for stored values. Found in review of BUG-07DNNY.'
priority: low
status: backlog
---
