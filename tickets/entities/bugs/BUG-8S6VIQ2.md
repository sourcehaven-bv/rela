---
id: BUG-8S6VIQ2
type: bug
title: rrule_next returns the after date itself for a rule without DTSTART
description: Lua rela.rrule_next lets rrule-go default DTSTART to time.Now(), so an occurrence date asked about on that day is returned again and recurring schedules stop advancing.
priority: high
effort: s
why1: luaRruleNext builds the rule without DTSTART, so rrule-go uses time.Now() and rule.After(<date> 00:00) returns <date> at the current clock time.
why2: The Lua helper (April 2026) parses the RRULE itself instead of using metamodel.NextRrule, which anchors a missing DTSTART at the after day.
why3: metamodel.NextRrule was added in August 2026 for predicates; the older Lua helper was not switched to it.
why4: The Lua tests use only rules with DTSTART, and the default-to-today test checks the date format, not the value.
why5: Two RRULE stepping implementations existed and no test compared them on a rule without DTSTART on its own occurrence date.
prevention: Lua rrule_next now delegates to metamodel.NextRrule, so there is one implementation. Tests pin a rule without DTSTART asked about on its own occurrence, including today.
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

`rela.rrule_next(rule, after)` returns `after` itself when `after` is an
occurrence of a rule without DTSTART. Atlas `scripts/recurrence.lua` calls
`rela.rrule_next(herhaling, volgende_datum)` after spawning a task, so for a
rule like `FREQ=YEARLY;BYMONTH=10;BYMONTHDAY=1` `volgende_datum` never advances.

`luaRruleNext` (internal/lua/date.go) parsed the rule itself and passed it to
`rrule.NewRRule` without a DTSTART. rrule-go then uses `time.Now()` as DTSTART,
so every occurrence carries the current clock time. `rule.After` on `<date>
00:00` finds `<date> <now>` and returns the same date. `metamodel.NextRrule`
anchors a missing DTSTART at the `after` day and does not have this defect; the
predicate `rrule_next()` already uses it.

## Reproduction

`rela.rrule_next("FREQ=DAILY", "<today>")` returns today instead of tomorrow.
`TestRruleNextOnTodaysOccurrence` and the no-DTSTART cases in `TestRruleNext`
(internal/lua) fail on develop.

## Fix

`luaRruleNext` calls `metamodel.NextRrule`, so Lua and predicates share one
implementation. An exhausted rule still returns nil; any other error is raised.
Tests cover a rule without DTSTART asked about on its own occurrence, and
DTSTART both as `;DTSTART=` and as a separate `DTSTART:` line.
