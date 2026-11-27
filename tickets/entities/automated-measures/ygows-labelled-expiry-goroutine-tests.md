---
id: ygows-labelled-expiry-goroutine-tests
type: automated-measure
title: 'Test: awareness expiry leak tests count only their own labelled goroutine'
description: The ygows awareness expiry leak tests count goroutines that carry a pprof label set by the test, not runtime.NumGoroutine.
kind: test
location: third_party/ygows/server_internal_test.go
status: active
---

createLabeledRoom, expiryGoroutines and waitExpiryGoroutinesGone require one labelled StartAutoExpiry goroutine before eviction or Shutdown and none after.
