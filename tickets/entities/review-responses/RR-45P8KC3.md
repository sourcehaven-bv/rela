---
id: RR-45P8KC3
type: review-response
title: 'Trigger ACL check: placeholder scanner duplicated'
finding: placeholderReads re-implemented the scan in triggers.expand; a change to one alone would let a read through.
severity: minor
resolution: triggers.PropertyReads runs the same expand scan and the checker uses it. TestPropertyReads_AgreesWithInterpolate pins agreement with Interpolate on awkward inputs.
status: addressed
---
