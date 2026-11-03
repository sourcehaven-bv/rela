---
id: RR-M70ORW
type: review-response
title: Justify the new FieldVerdictResolver method and pin the Demo fixture
finding: Record in the godoc why the method lives on FieldVerdictResolver (the answer must come from the resolver that redacts; an optional capability would fail open) and test that the Demo resolver's answer matches its own FieldVerdicts.
severity: minor
resolution: FieldVerdictResolver godoc says why the method is required rather than an optional capability (a missing method would default to visible). TestDemoResolver_FieldVisibleOnEveryRowMatchesVerdicts pins the demo fixture against its own FieldVerdicts.
status: addressed
---
