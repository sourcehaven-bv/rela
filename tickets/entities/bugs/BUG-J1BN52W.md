---
id: BUG-J1BN52W
type: bug
title: TestHiddenProperty_NotCompared fails when a random entity id contains 9
description: TestHiddenProperty_NotCompared in internal/mapimport asserts the plan JSON does not contain the substring "9". The plan error text includes the generated entity id (e.g. CTL-NI9I), so the test fails whenever the random id contains a 9.
priority: medium
effort: xs
why1: The test asserts the marshalled plan does not contain the substring "9", and the plan error text contains the generated entity id, which sometimes has a 9 (CTL-NI9I).
why2: 'The hidden value was the single digit 9, which is not distinctive: any digit 9 anywhere in the plan matches it.'
why3: The entity is created without a fixed id, so the id is random and the failure shows only in about one run in ten.
why4: A substring check on serialized output was used as a leak check without a sentinel that cannot occur in other plan text.
why5: Leak assertions in tests are written ad hoc per test; there is no convention that the searched value must be a unique sentinel.
prevention: Both hidden-property tests share assertHidden with an 8-digit sentinel (hiddenWeight) that no generated id can contain, and the fix was checked with -count=200.
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Reproduction

Run `go test -count=200 -run TestHiddenProperty_NotCompared
./internal/mapimport/`; some runs fail with the id `CTL-NI9I` in the plan.

## Root cause

The hidden value is the integer 9 and the assertion is `strings.Contains(json,
"9")`, so any digit 9 in the random id trips it.
