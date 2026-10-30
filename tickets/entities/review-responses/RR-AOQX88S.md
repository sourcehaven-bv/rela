---
id: RR-AOQX88S
type: review-response
title: Test counts only untyped full scans
finding: fullScans in idlisting_test.go ignores a scan that passes a type.
severity: nit
reason: The guard test in the internal test file covers the real regression path.
status: wont-fix
---
