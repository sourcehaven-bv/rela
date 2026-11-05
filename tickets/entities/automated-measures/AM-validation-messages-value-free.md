---
id: AM-validation-messages-value-free
type: automated-measure
title: Built-in validation messages never quote the value
description: A table test over every property type feeds a value with a marker and asserts no ValidationError message contains it. A new check that quotes its value fails the test.
kind: test
location: internal/metamodel/validation_novalue_test.go (TestValidatePropertyValue_MessageNeverQuotesValue)
status: active
---
