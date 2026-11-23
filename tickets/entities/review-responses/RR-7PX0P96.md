---
id: RR-7PX0P96
type: review-response
title: Guide overstates the unset-is-false rule
finding: 'Code review: an unset date against today() still errors under LoweringSemantics and is logged; the guide did not say so.'
severity: minor
resolution: GUIDE-desktop now says a missing or invalid date does not match and is written to the debug log.
status: addressed
---
