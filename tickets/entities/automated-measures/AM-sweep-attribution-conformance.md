---
id: AM-sweep-attribution-conformance
type: automated-measure
title: Every versioning backend credits swept create/update versions to the editor
description: storetest.RunSweepAttributionTests runs for every backend that declares Capabilities.Versioning, which must also supply a SweepNow driver. It asserts attributed writes are credited to their editor and unattributed ones to version-sweep, for entities and relations.
kind: test
location: internal/store/storetest/sweepattribution.go
status: active
---
