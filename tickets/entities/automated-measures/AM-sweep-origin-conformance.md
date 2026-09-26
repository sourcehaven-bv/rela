---
id: AM-sweep-origin-conformance
type: automated-measure
title: Every versioning backend records copy provenance on swept versions
description: A storetest case that writes with store.WithOrigin, sweeps, and asserts the captured version carries the origin, run for every backend that declares Capabilities.Versioning.
kind: test
location: internal/store/storetest (added in the BUG-YC5Z07 fix PR)
status: proposed
---
