---
id: statefamily-relation-order-test
type: automated-measure
title: 'Test: a partially failed cascade removes and reports a deterministic relation'
description: 'Regression for BUG-CMG6Q7: injects a remove failure on the second relation file and asserts the partial DeleteResult names exactly the first one (SOL-1), so the cascade visit order is stable. Fails if FSStore.stateFamily reverts to returning `related` in s.relations map order. Run under -shuffle=on on both the default and sqlite build tags.'
kind: test
location: internal/store/fsstore/recovery_test.go (TestDeleteEntity_PartialCascade_ReportsWhatWasRemoved)
status: active
---

## What it prevents

`stateFamily` builds `related` by ranging over the `s.relations` map, which Go
randomizes. The cascade deletes relation files in that order, so a cascade that
aborts partway removed — and reported — a different relation on each run.

This is the same defect as BUG-ULU7X6 in `LinearSearch`, one layer down: a map
feeding a slice whose order something downstream observes.

## Why it needs a partial failure to catch

While every relation is deleted, order is unobservable — any order yields the
same final state, and the test suite was blind to it. Only once a cascade can
abort midway does the visit order become a visible, assertable fact. A test
that merely deletes successfully cannot catch a regression here.

## Verifying it still holds

The assertion is order-sensitive by construction, so re-running is the check:

```bash
go test -run TestDeleteEntity_PartialCascade -count=30 ./internal/store/fsstore/
go test -tags sqlite -shuffle=on ./internal/store/...
```

A regression shows up as an intermittent "should have 1 item(s), but has 0" —
so treat any flake in this test as an ordering bug, never as a retry candidate.
