---
id: RR-YLMER3S
type: review-response
title: E2E overflow check can pass on no match
finding: sectionValueOverflows returns false when no value matches, and the test did not check the value is shown.
severity: minor
resolution: 'Addressed: the helper throws on no value, and the spec checks the value is visible and wraps onto more than one line in both sections.'
status: addressed
---
