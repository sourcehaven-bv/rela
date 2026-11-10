---
id: RR-VPP5IQH
type: review-response
title: Missing range index gives a confusing error
finding: A 422 without a range member reported a range the request does not hold.
severity: minor
resolution: With one relation range in the request the client uses it; otherwise it fails with a protocol error saying the remote did not name the range.
status: addressed
---
