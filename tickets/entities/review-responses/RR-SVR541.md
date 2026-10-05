---
id: RR-SVR541
type: review-response
title: TestCompiled_Default is not parallel
finding: The new test and subtests lack t.Parallel().
severity: nit
reason: No test in worlds_test.go uses t.Parallel(); the new test follows the file's convention.
status: wont-fix
---
