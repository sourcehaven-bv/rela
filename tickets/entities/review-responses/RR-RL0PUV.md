---
id: RR-RL0PUV
type: review-response
title: Weak large-upload and concurrency tests
finding: The HTTP test sent 5 MiB only; the concurrency test could t.Fatalf from goroutines.
severity: nit
resolution: HTTP and stdio tests now send MaxUploadBytes (this found an off-by-rounding size precheck that refused a file exactly at the limit; fixed and boundary-tested); the concurrency test collects errors and asserts on the test goroutine.
status: addressed
---
