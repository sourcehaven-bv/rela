---
id: BUG-X79VL08
type: bug
title: 'TestHiddenProperty_NotCompared flakes: leak probe matches a digit of a generated id'
description: 'TestHiddenProperty_NotCompared in internal/mapimport failed about 1 in 9 runs, with or without -race. It hid weight 9 on a control with a generated id and failed if the plan JSON contained the character 9. The plan''s error message names the control by that id (e.g. CTL-39R1), so any id with a 9 failed the check although nothing leaked. Fix: seed the control with the fixed id CTL-1 and hide 987654.'
priority: medium
effort: xs
why1: The test failed whenever the marshalled plan contained the character 9. The plan's error message names the control by its generated id, and about one id in nine contains a 9.
why2: The test used the single digit 9 as both the hidden value and the leak probe, and seeded the control with a generated id that also appears in the plan.
why3: A substring check over the whole plan JSON is only sound if the probe cannot occur in any other field, and nothing made the author check which other text the plan carries.
why4: The sibling test TestHiddenProperty_RealPolicy already used a fixed id and the value 987, but the two tests were written separately and the pattern was not shared.
why5: Leak tests that grep serialized output have no helper or convention requiring a distinctive probe value and deterministic fixtures. The brief blamed -race, but the failure is random-id dependent and reproduces without it.
prevention: 'P2: leak tests that grep serialized output seed fixed ids and use a hidden value of several digits that cannot occur in any other field, as both hidden-property tests in internal/mapimport now do. P3: when a test flakes, reproduce it without -race before blaming concurrency; this one failed at the same rate either way.'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Description

`TestHiddenProperty_NotCompared` (`internal/mapimport/review_test.go`) failed
about 1 in 9 runs, with or without `-race`. It seeded a control with the hidden
value `weight: 9` under a generated id, then failed if the marshalled plan
contained the character `9`. The plan's error message names the control by its
generated id (for example `CTL-39R1`), so any id containing a 9 failed the
check. Nothing leaked; the plan was correct.

## Fix

Seed the control through the store with the fixed id `CTL-1` and hide the
six-digit value `987654`, matching `TestHiddenProperty_RealPolicy`. The leak
check now searches for that value.
