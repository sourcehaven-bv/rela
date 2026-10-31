---
id: RR-O8U5GKF
type: review-response
title: archplan baseline still names TreeValue a configure type
finding: .archplan/baseline.json describes TreeValue as a type of web/configure, which is false after the move.
severity: nit
reason: The baseline is generated. It is refreshed by the archplan export, which needs a person to run archplan trust on this machine first; a hand edit would be overwritten.
status: deferred
---
