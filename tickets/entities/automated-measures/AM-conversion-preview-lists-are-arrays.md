---
id: AM-conversion-preview-lists-are-arrays
type: automated-measure
title: Conversion preflight lists are arrays and the dialog shows the review step
description: Asserts that the preflight response never sends null list fields, that the client normalizes them, and that Review then Convert works in a browser.
kind: test
location: internal/dataentry/conversions_handler_test.go; frontend/src/components/entity/ConversionModal.test.ts; e2e/tests/conversion-review.spec.ts
status: active
---
