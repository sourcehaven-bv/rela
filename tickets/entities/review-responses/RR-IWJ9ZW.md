---
id: RR-IWJ9ZW
type: review-response
title: readableFaceOf passes a zero visibility.World
finding: viewworld.go passed visibility.World{} to refIn (cranky review).
severity: nit
resolution: It passes defaultWorldHandle().visibility() (see RR-BQIO69).
status: addressed
---
