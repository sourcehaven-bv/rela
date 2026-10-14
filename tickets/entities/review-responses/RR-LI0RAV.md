---
id: RR-LI0RAV
type: review-response
title: validOp in docs is a second closed list of ops
finding: internal/docs/assert_acl.go validOp lists the ops again and can miss a new one; Ops() allocated on every valid check.
severity: minor
resolution: Op.Valid is exported and validOp calls it. Ops are a package array; Valid scans it without allocating and Ops returns a copy (TestOps_ReturnsACopy).
status: addressed
---
