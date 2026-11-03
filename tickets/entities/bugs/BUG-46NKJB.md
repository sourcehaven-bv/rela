---
id: BUG-46NKJB
type: bug
title: 'TestConvert_RefusesToCopyAnUnreadableField/no_value flakes: map order runs Convert before Preview'
description: The subtest ranges over a map of Preview and Convert; when Convert runs first it removes NOTE-1 and Preview fails with entity not found.
priority: medium
effort: xs
why1: Preview returned entity not found for NOTE-1 because Convert had already run and removed the source.
why2: The subtest stores Preview and Convert in a map[string]func() error and ranges over it; Go randomizes map iteration order, so Convert runs first in about half the runs.
why3: In the no value case the conversion is allowed, so Convert succeeds and mutates the shared fixture. In the hidden value case both calls are refused and order does not matter, which hid the dependency.
why4: A map literal is a common idiom for naming a set of calls in a test. It is safe only when the calls are independent; here one call mutates state the other reads.
why5: Nothing flags a map of calls whose order matters. The test passed on most local runs, so review did not catch it; the failure only shows with repeated runs.
prevention: 'Use an ordered slice when calls in one loop share a fixture and any call mutates it. Checked the package and the test files touched by PR #1895 and #1907: this was the only map of calls where a call mutates shared state.'
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

## Description

`TestConvert_RefusesToCopyAnUnreadableField/no_value`
(`internal/entitymanager/conversion_test.go`) fails about 15% of runs:

```
conversion_test.go:659: preview: entity not found: NOTE-1
```

The subtest puts `Preview` and `Convert` in a `map[string]func() error` and
ranges over it. Go map iteration order is random. In the `no value` case the
conversion is allowed, so when `Convert` runs first it succeeds and removes
NOTE-1, and the `Preview` that follows finds no source.

## Reproduction

```
go test -race -run TestConvert_RefusesToCopyAnUnreadableField -count=50 ./internal/entitymanager
```

8 of 50 runs fail on develop (59f68c58e).

## Fix

Run the calls from an ordered slice, `Preview` before `Convert`, so the
non-mutating call always sees the seeded source.

The same change reached develop in #1926 (commit 46e3389ef, "entitymanager:
preview before convert in test") while this fix was in review, so the code here
is develop's version.
