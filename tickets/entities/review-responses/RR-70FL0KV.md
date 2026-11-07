---
id: RR-70FL0KV
type: review-response
title: 'Design: decorated stores hide the capability'
finding: Store decorators embed store.Store and hide extra methods, so a type assertion in step 5 can fail at request time.
severity: minor
resolution: The reader is a separate value built at wiring (pgstore.NewElementRanges, sqlitestore.NewElementRanges, elementnaive.New), not a store method, so no decorator can hide it and no type assertion is needed.
reason: 'Step 5 wiring: assert once at construction through store.Innermost and fail the constructor. Counting forwards both methods here.'
status: addressed
---
