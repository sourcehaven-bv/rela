---
id: RR-6SL5UT
type: review-response
title: 'TestClone_AllowsPathInsideBaseDir made a live request to github.com'
finding: |-
    The positive containment case drove a real Clone. Containment passes by
    construction there, so nothing stopped it proceeding to the actual git fetch:
    the test made an unauthenticated request to github.com on every run (~0.21s,
    returning "authentication required: Repository not found").

    Its comment claimed it "still fails later (no network / not a real repo)", which
    assumes no network. On a machine WITH network it takes a different code path
    than a CI runner with egress blocked, and is flaky whenever GitHub is.
severity: significant
resolution: |-
    Rewritten against containedPath directly, asserting it returns the cleaned path
    with a nil error. Runtime 0.21s -> 0.00s, and no test in the package now touches
    the network (verified: every TestClone_* case reports 0.00s).

    The NEGATIVE cases still go through Clone deliberately. There the claim is about
    the BOUNDARY -- that a bad path is rejected before anything happens -- so the
    test has to cross it. Here the claim is only "this path is judged contained",
    which is exactly what containedPath returns, so the network call added risk and
    no coverage.
status: addressed
---

## Resolution

Rewritten against containedPath directly, asserting it returns the cleaned path
with a nil error. Runtime 0.21s -> 0.00s, and no test in the package now touches
the network (verified: every TestClone_* case reports 0.00s).

The NEGATIVE cases still go through Clone deliberately. There the claim is about
the BOUNDARY -- that a bad path is rejected before anything happens -- so the
test has to cross it. Here the claim is only "this path is judged contained",
which is exactly what containedPath returns, so the network call added risk and
no coverage.
