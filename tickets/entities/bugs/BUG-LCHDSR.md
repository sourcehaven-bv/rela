---
id: BUG-LCHDSR
type: bug
title: SQL graph backends diverge from graphquerynaive on headless closures, non-string lists and reals
description: 'Divergences between the SQL graph backends and graphquerynaive found in the TKT-B51CYD review: pgstore''s entity closure skips headless entities and seeds from the whole type; list equality ignores non-string elements; reals render differently.'
priority: medium
effort: m
status: backlog
---

## Description

The code review of TKT-B51CYD found divergences between the SQL graph backends
and graphquerynaive that the differential harness does not generate yet:

1. **Headless entities in pgstore's entity closure.** pgstore seeds `EntityInheritThrough` from `e0.face = ''` rows only (the TKT-WAV8XP Q5 "identity anchor"). A faced type may store no default-face row, so such an entity never matches on postgres, while graphquerynaive and sqlitestore match it. This path is shared with the ACL and visible search, so the fix needs a design check.
2. **pgstore's entity closure seeds from the whole type** even for a one-page `MatchingIDs`. sqlitestore seeds from the page's ids since TKT-B51CYD; pgstore should do the same.
3. **List equality with non-string elements.** `tags: [1, 2]` with `tags == "1"` matches in graphquerynaive (it compares `Stringify(item)`) but not in either SQL backend, which match text elements only.
4. **Real formatting.** SQL renders a real as its JSON token (`0.000001`); graphquerynaive uses `fmt.Sprint` (`1e-06`), so equality and range filters disagree.

When fixed, extend `storetest.RunGraphDifferential` with headless entities,
lists of integers and booleans, and reals such as `0.000001` and `1e21`.
