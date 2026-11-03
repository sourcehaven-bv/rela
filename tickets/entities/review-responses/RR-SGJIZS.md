---
id: RR-SGJIZS
type: review-response
title: The test name does not match the function it tests
finding: "TestLocalPrincipal (internal/cli/identity_test.go:25) tests newLocalIdentity. The plan's name localPrincipal no longer exists.\tA grep for the function's test finds nothing.\tRename it TestNewLocalIdentity."
severity: nit
resolution: Renamed to TestNewLocalIdentity.
reason: Renamed to TestNewLocalIdentity.
status: addressed
---
