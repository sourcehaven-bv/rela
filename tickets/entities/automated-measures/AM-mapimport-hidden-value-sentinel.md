---
id: AM-mapimport-hidden-value-sentinel
type: automated-measure
title: Hidden-property leak check uses a sentinel no entity id can contain
description: assertHidden marshals the import plan and fails if it contains hiddenWeight (an 8-digit value) or compares the hidden property. The sentinel is longer than any generated id, so a random id cannot trip the check.
kind: test
location: internal/mapimport/review_test.go (assertHidden, TestHiddenProperty_NotCompared, TestHiddenProperty_RealPolicy)
status: active
---
