---
id: RR-4TB9BZ
type: review-response
title: Busy duration never advances on an open board
finding: 'frontend/packages/rela-components/src/components/feedback/RlBusyIndicator.vue:38-61 computes the duration with Date.now() inside computed(), whose only reactive dependency is props.entries. Vue caches the value, so "Planning · <1m" stays as it is until the parent passes a new entries array. A board only refetches on a graph write (live) or a reload, so a run that stalled hours ago still reads "<1m". The plan (Q3) names the visible time in state as the stall signal, so the signal is wrong exactly when it matters. The godoc at lines 15-16 ("it moves on whenever the indicator renders again") and the plan''s "recomputed on render" overstate this, because a re-render does not re-evaluate a cached computed. Fix: add a shared minute ticker (a module-level `now` ref updated by one setInterval of 60s, started on first use and stopped when the last subscriber unmounts) in rela-components, read it inside `text` and `name`, and add a vitest with fake timers that advances 2h and asserts the text changes from "3h" to "5h".'
severity: significant
resolution: Added useMinuteClock (one shared setInterval at module level, a ref per caller, timer stops with the last scope); RlBusyIndicator reads it in its computed so the time moves each minute; godoc fixed; vitest with fake timers checks 3h becomes 5h and that one timer starts and stops.
status: addressed
---
