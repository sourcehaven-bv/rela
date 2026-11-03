---
id: RR-EFXP8Q
type: review-response
title: MapAll does not strip everyone face grants
finding: stripEveryoneBaseline handles Baseline but not Faces, so everyone face grants repeat per principal.
severity: minor
reason: Affects only the whole-graph inventory output, which --expect and --webhook do not use. The duplication over-reports and does not hide access.
status: deferred
---
