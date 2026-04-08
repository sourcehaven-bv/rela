---
id: RR-2RJW
type: review-response
title: 'F15: TestProvider_Chat_ResponseTooLarge encodes the bug instead of testing the fix'
finding: The test asserted aiErr.Kind != ErrBadResponse && aiErr.Kind != ErrNetwork — accepting either classification. 'Either is fine' tests pass even if the implementation is broken in one of the two directions.
severity: minor
resolution: Tightened the test (folded into the F4 fix) to assert exactly ErrBadResponse and to verify the message contains 'exceeded'. The dual-classification escape hatch is gone.
status: addressed
---
