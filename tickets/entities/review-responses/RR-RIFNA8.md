---
id: RR-RIFNA8
type: review-response
title: Text form of reals and enum rank over raw ->>
finding: ->> returns typed values; CAST(REAL) may not match fmt.Sprint; enum CASE over raw ->> misses integer-stored values.
severity: minor
resolution: txt() returns the JSON token (->) for reals; enum rank and ordering use txt(); reals outside the common range and lists stay out of ordered differential cases.
status: addressed
---
