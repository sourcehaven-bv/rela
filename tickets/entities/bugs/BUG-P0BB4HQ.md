---
id: BUG-P0BB4HQ
type: bug
title: One broken view condition disables every list condition
description: 'If one list or kanban condition: fails to compile, viewCondition returns nil for every view, so all lists show all rows. The server logs nothing.'
priority: high
effort: s
status: ready
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
