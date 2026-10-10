---
id: AM-dataentry-422-hides-backend-errors
type: automated-measure
title: 'Test: dataentry catch-all 422s carry only typed validation errors'
description: Planned for BUG-E3V44J. Extends TestNoInternalErrorDetail to the 422 catch-alls and adds handler tests that inject a store fault and assert a 500 with check server logs instead of a 422 carrying the fault text.
kind: test
location: internal/dataentry/internalerror_test.go (planned)
status: proposed
---
