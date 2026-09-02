---
id: RR-BPGG9C
type: review-response
title: 'Making BaseDir required silently gutted TestClone_PathExists'
finding: |-
    TestClone_PathExists constructed CloneOptions with no BaseDir. Making the field
    required meant that test now fails at the new containment guard and never
    reaches the os.Stat "path already exists" check it exists to cover. Its
    assertion was a bare `err == nil` check, so it kept PASSING while testing
    nothing it claimed to test -- leaving the os.Stat guard with zero coverage.

    This is the same failure mode I had explicitly reasoned about for my own new
    tests (that Clone fails for many environmental reasons, so a bare non-nil
    assertion proves nothing) -- and then walked straight into on the existing test
    directly above them.
severity: critical
resolution: |-
    Added `BaseDir: dir` so the call reaches os.Stat again, and an assertion on the
    error text ("path already exists") so it cannot pass on an unrelated failure a
    second time.

    Mutation-verified: deleting the os.Stat branch from Clone now reddens
    TestClone_PathExists. Before the fix it stayed green.

    The general lesson is worth more than the fix: tightening a precondition can
    silently relocate an existing test's failure point. Any test constructing the
    changed type needs RE-READING, not just re-running -- a green suite is exactly
    what this looks like.
status: addressed
---

## Resolution

Added `BaseDir: dir` so the call reaches os.Stat again, and an assertion on the
error text ("path already exists") so it cannot pass on an unrelated failure a
second time.

Mutation-verified: deleting the os.Stat branch from Clone now reddens
TestClone_PathExists. Before the fix it stayed green.

The general lesson is worth more than the fix: tightening a precondition can
silently relocate an existing test's failure point. Any test constructing the
changed type needs RE-READING, not just re-running -- a green suite is exactly
what this looks like.
