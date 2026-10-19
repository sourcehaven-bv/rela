---
id: AM-conversion-read-gate-ordered-steps
type: automated-measure
title: Conversion read-gate test runs Preview before Convert
description: The subtest runs its Preview and Convert steps from an ordered slice, so an allowed Convert cannot remove the source before Preview reads it.
kind: test
location: internal/entitymanager/conversion_test.go (TestConvert_RefusesToCopyAnUnreadableField)
status: active
---
