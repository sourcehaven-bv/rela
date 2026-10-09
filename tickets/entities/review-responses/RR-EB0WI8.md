---
id: RR-EB0WI8
type: review-response
title: Helper test does not check the cause is logged
finding: TestWriteInternalError_HidesTheCause only checked the response.
severity: minor
resolution: The test captures slog output and asserts the cause is logged.
status: addressed
---
