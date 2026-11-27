---
id: BUG-P0BB4HQ
type: bug
title: One broken view condition disables every list condition
description: 'If one list or kanban condition: fails to compile, viewCondition returns nil for every view, so all lists show all rows. The server logs nothing.'
priority: high
effort: s
why1: viewCondition returned nil for every view when the compiler reported any problem.
why2: The compiler returned one lookup for all views and dropped it on the first error instead of marking only the broken view.
why3: 'Fail-open was the default: a missing matcher meant no filter rather than an error.'
why4: Startup validation rejects a broken condition, so the fail-open path was only reachable after a hot reload, which no test covered.
why5: No test exercised a broken condition next to a good one, so the all-or-nothing lookup was never challenged.
prevention: Each view's matcher is now resolved per view and a declared condition without a matcher fails that view closed. Tests pin a broken and a good condition side by side.
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

## Description

A list or kanban `condition:` that fails to compile made `viewCondition`
(internal/dataentry/viewcondition.go) return nil for every view, because the
compiler returned no lookup when it had any problem. Every list then showed
every row, with nothing logged. Found while building an OmniFocus-style example
project.

## Fix

Compile conditions per view; a view whose declared condition has no compiled
matcher fails closed (HTTP 500, export fails, nav and page markers skipped).
Problems are logged once per config load.
