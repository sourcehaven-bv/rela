---
id: BUG-33B3JCH
type: bug
title: Two-tab autosave property test misses its out-of-attempts floor at random
description: The exhaust fault spread its 412s over other saves, so the exhausted total sat near its floor and an unseeded run fell under it now and then.
priority: medium
effort: xs
why1: The exhausted total over 300 random runs fell to 29, under the floor of 30.
why2: fc.assert ran without a seed, so each CI run drew a new sample, and the mean total (about 40) sat only about 2.5 standard deviations above the floor.
why3: The exhaust fault armed a shared count of three empty 412s that any PATCH spent, including the other tab's saves and the same tab's other channel, so the 412s were often split over two saves and none ran out of attempts.
why4: The fault was written per tab, but one tab runs property and body saves apart, and nothing checked that an armed exhaust exhausted a save.
why5: Floors over random samples were set by feel without measuring their spread, and a floor failure did not name the seed, so it could not be replayed.
prevention: The exhaust fault binds to one save; the floor run takes PROP_SEED and names its seed in every floor message, so a low draw is replayable.
started: "2026-11-07"
completed: "2026-11-07"
status: done
severity: low
---

## Symptom

`useAutoSave.twoClients.prop.test.ts` › "merges what can be merged and reports
the rest" failed three times in one evening (PR 2110 in the merge queue, PR
2113's checks) with `saves out of attempts: expected 29 to be greater than or
equal to 30`. Each passed on rerun.

## Cause

The floors bound totals over a random sample: `fc.assert` ran without a seed, so
every CI run drew new command sequences. Over 15 local runs of 300 the
`exhausted` total ranged 34 to 50 (mean about 40) against a floor of 30, so a
low draw falls under it now and then.

The mean was that low because the `exhaust` fault did not do what its comment
said. It armed a shared counter of three empty 412s that any PATCH spent: the
other tab's saves, and the same tab's save on its other channel (property and
body saves run apart). So the three 412s were often split over two saves and
none ran out of attempts.

Not every exhausted save is reported, by design: autosave does not report a
failed save whose field was edited again meanwhile. The count stays random.

## Fix

- The exhaust fault binds to the save whose request it first hits (each
`runWrite` call is tagged with a save number) and answers only that save's
attempts. Other saves pass through untouched. A save that ends early releases
its binding.
- The floor test runs with a seed and names it and the run count in each
floor's message, so a low draw can be replayed with `PROP_SEED` and
`PROP_RUNS`.

The threshold is unchanged.

## Evidence

Seeds 1 to 200, 300 runs each, `exhausted` total against the floor of 30:

| Code | Min | Mean | Seeds under 30 |
|---|---|---|---|
| old | 23 | 39.8 | 7 |
| new | 46 | 64.5 | 0 |
