---
id: BUG-IY5RC15
type: bug
title: Two-tab autosave mutant test rejects one of the two blind-write reports
description: The body-merge mutant test expected 'overwrote c:' only; a run that caught the fault as 'wrote c:… which it does not own' failed it.
priority: medium
effort: xs
why1: 'The merge-queue Frontend job failed: no violation matched ''overwrote c:'' although the property had failed.'
why2: Every failing run reported the blind write as 'wrote c:… which it does not own' instead of 'overwrote c:… which it was never shown'.
why3: checkWrite reports one blind write as either message depending on what the tab was shown, and the unseeded property sometimes draws only runs where the tab had been shown the value it overwrote.
why4: The assertion was written against the message a few observed runs produced, not against the check that catches the fault.
why5: Mutant tests over an unseeded random property pinned one wording of a violation that the checker can report in more than one way, and nothing ran them often enough to see the other wording.
prevention: The mutant tests now match any blind-write report for the field kind (a regex over both checkWrite messages), and the comment above them states that either report catches the fault.
started: "2026-11-12"
completed: "2026-11-12"
status: done
severity: low
---

## Symptom

The merge-queue Frontend job for PR #2201 (run 43078734201) failed on
`useAutoSave.twoClients.prop.test.ts` > "catches a body merge that always writes
ours". The retry passed with no code change.

## Root cause

The test breaks `mergeText` so every body merge writes the tab's own text, then
expects the property to fail with a message matching `overwrote c:`.
`checkWrite` reports one blind write in two ways: the tab wrote a value it does
not own, or it overwrote a value it was never shown. Which one fires depends on
what the tab was shown in that run. The property runs without a seed, so now and
then every failing run reports only "wrote c:… which it does not own". The fault
was caught, but the assertion did not accept that report.

The property-merge test had the same narrow assertion (`overwrote p:`).

## Fix

Both mutant tests assert that a blind write to the field kind was reported, with
either message. In 15 instrumented runs of both mutant tests, every run reported
a blind write to the broken field kind.
