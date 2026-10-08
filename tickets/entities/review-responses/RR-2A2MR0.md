---
id: RR-2A2MR0
type: review-response
title: Parent-of-temp-dir test is weak on Linux CI
finding: With TMPDIR unset filepath.Dir("/tmp") is /, already refused as /, so the parent-of-temp-dir case is never exercised on Linux. Set TMPDIR to a nested dir and assert its parent is rejected.
severity: minor
resolution: The rejection test sets TMPDIR to a nested directory under t.TempDir() and asserts both it and its parent are refused, so the parent-of-temp-dir case runs on Linux CI.
status: addressed
---
