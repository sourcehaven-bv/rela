---
id: RR-GDE3PY
type: review-response
title: AC8's 'suite passes unchanged' is unachievable and pressures the implementer to loosen assertions
finding: 'AC8 requires the existing useAutoSave.test.ts to pass ''unchanged''. This cannot hold: the plan changes the observable request shape of every autosave PATCH — the three call sites currently pass a literal undefined for the etag argument (useAutoSave.ts:319,370,416) and Step 1 replaces it with a real value. Any existing assertion on store.update call arguments (the suite mocks at exactly that Pinia seam) will either break, or silently keep passing while asserting nothing about the new header. An AC mandating ''unchanged'' pressures the implementer toward the second outcome — which is how AC2 (If-Match sent) ends up untested where it matters. FIX: AC8 should require existing BEHAVIORAL guarantees (FIFO ordering, debounce coalescing, no-op suppression, warning routing, the S5 invariant) to hold, with the suite updated where it asserts on call shape; state explicitly that etag assertions must be TIGHTENED, not relaxed.'
severity: minor
status: open
---
