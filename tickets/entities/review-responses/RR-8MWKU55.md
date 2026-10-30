---
id: RR-8MWKU55
type: review-response
title: Acceptance criteria do not pin the validate exit status
finding: AC1 tests CheckDataEntry through ValidateWithFS but does not assert that the result counts as an error, which is what makes rela validate exit non-zero.
severity: nit
resolution: 'Plan updated: the test asserts result.HasErrors() and DataEntryValid=false.'
status: addressed
---
