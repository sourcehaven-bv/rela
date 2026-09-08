---
id: RR-1S9AX5
type: review-response
title: The 'falls back to the unpushed query' subtest asserted only a call count
finding: The third subtest of TestNextAction_ConditionPrefiltersReachTheStore asserted Prefilters was called once — already established by the first subtest — and never checked a suggestion, so it would pass whether or not the empty pre-filter fell back correctly. The subtests also share one app and swap its matcher, which must stay sequential.
severity: nit
resolution: The subtest now asserts a suggestion is returned and is one of the two seeded candidates (both eligible when nothing is pushed and Match accepts all); a comment on the parent test says the subtests must not be parallelised.
status: addressed
---
