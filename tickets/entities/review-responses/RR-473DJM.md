---
id: RR-473DJM
type: review-response
title: DeepEqual treats nil and empty value lists as different
finding: Loops over calls with no values could be marked elided.
severity: nit
resolution: equalSteps compares with slices.Equal.
status: addressed
---
