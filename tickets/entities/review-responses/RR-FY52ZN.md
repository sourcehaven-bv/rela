---
id: RR-FY52ZN
type: review-response
title: Test did not show the 403 came from the field rule
finding: The test asserted the status code only, so an ACL deny would also pass it.
severity: minor
resolution: The test asserts rule_id field-affordance:read-only:screenshot for upload and delete.
status: addressed
---
