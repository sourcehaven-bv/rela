---
id: RR-B9ABNX7
type: review-response
title: Badge test leaks its shell on failure
finding: The shell was unmounted only at the end of the test.
severity: nit
resolution: The shell is unmounted in afterEach.
status: addressed
---
