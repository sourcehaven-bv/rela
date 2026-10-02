---
id: RR-0UMG10
type: review-response
title: faceNeighbors error path is untested
finding: The error returned by faceNeighbors reaches writeGateError but no test drives it.
severity: minor
resolution: 'Added TestExport_NeighborFaultFailsTheExport: a failing neighbor read gives a 500 and no content.'
status: addressed
---
