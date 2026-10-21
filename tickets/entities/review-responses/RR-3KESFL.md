---
id: RR-3KESFL
type: review-response
title: Vacuous unique-check test
finding: FailedChecksLeaveNoEntity/unique never triggers a unique conflict and asserts success.
severity: minor
resolution: The unique test now uses a recipe that sets a duplicate code and asserts a unique violation, one task and no relation record.
status: addressed
---
