---
id: RR-8DW6KRP
type: review-response
title: capture builds the whole write-deps bundle for Records
finding: capture calls ScheduledLuaWriteDeps() and uses only .Records.
severity: minor
reason: ScheduledLuaWriteDeps is the scheduler's existing consumer-side workspace interface, which every job kind already reaches its write deps through; a second appender path would duplicate the wiring in rela-server, rela-desktop and the tests for no change in authority.
status: wont-fix
---
