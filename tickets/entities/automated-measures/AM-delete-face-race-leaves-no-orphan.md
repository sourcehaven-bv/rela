---
id: AM-delete-face-race-leaves-no-orphan
type: automated-measure
title: Delete of one face racing a create on another leaves no orphaned face
description: TestDeleteEntity_RacingStateCreateLeavesNoHeadlessFace drives a delete of one face concurrently with a create on another face of the same family and asserts the store is left consistent. Currently skipped pending BUG-22XSH3; removing the skip is the verification.
kind: test
location: internal/store/pgstore/deleterace_test.go
status: proposed
---

## Measure

`TestDeleteEntity_RacingStateCreateLeavesNoHeadlessFace`
(`internal/store/pgstore/deleterace_test.go`) drives a delete of one face
concurrently with a create on another face of the same entity family and asserts
the store is left consistent.

## Status

Currently `t.Skip("BUG-22XSH3")`. The test is written and reproduces the race;
it is skipped until the bug is fixed, at which point removing the skip is the
whole verification.

## Why this shape

The race was found by running the pgstore conformance suite against a real local
PostgreSQL rather than the DB-gated skip, which is what hid it. The measure
therefore lives in the pgstore suite, where `just test-postgres` runs it, rather
than as a unit test against a backend that cannot exhibit the interleaving.
