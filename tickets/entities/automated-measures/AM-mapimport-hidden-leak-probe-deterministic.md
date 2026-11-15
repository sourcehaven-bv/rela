---
id: AM-mapimport-hidden-leak-probe-deterministic
type: automated-measure
title: Hidden-property leak test uses a fixed id and a distinctive value
description: TestHiddenProperty_NotCompared seeds control CTL-1 with hidden weight 987654 and fails if that value appears in the plan JSON; no generated id can match the probe.
kind: test
location: internal/mapimport
status: active
---
