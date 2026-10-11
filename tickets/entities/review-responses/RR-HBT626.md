---
id: RR-HBT626
type: review-response
title: CI install hardcodes linux-amd64
finding: An arm64 runner would get an amd64 binary and fail at exec time, looking like a test failure.
severity: nit
resolution: The install step maps RUNNER_ARCH to amd64 or arm64 with a pinned SHA-256 each, and fails clearly on other arches.
status: addressed
---
