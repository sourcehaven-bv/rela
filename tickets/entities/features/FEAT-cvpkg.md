---
id: FEAT-cvpkg
type: feature
title: Cross-package Go coverage measurement
summary: go test -coverpkg=./... so coverage from consumer tests counts toward utility/library packages.
description: Run Go coverage with -coverpkg=./... so statements executed by tests in any package contribute to coverage for every file in the module. Utility packages and shared test kits are tracked honestly by the ratchet instead of showing 0% for lack of a local _test.go.
status: implemented
---

## Description

By default `go test` only attributes coverage to statements executed by tests
in the same package. Utility packages (e.g. `storeutil`) and shared test kits
(e.g. `storetest`) therefore register 0% coverage even when they are
extensively exercised by consumer tests in `fsstore`/`memstore`.

Adding `-coverpkg=./...` to the coverage invocation attributes coverage
across the whole module, giving honest numbers for library code and letting
the ratchet work correctly on packages that intentionally have no local
`_test.go`.
