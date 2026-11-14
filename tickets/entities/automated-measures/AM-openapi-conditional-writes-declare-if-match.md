---
id: AM-openapi-conditional-writes-declare-if-match
type: automated-measure
title: 'Test: every OpenAPI write operation whose handler honors If-Match declares the header and 412'
description: 'Guards against BUG-1Z6O2IE (GH-2228): the entity and relation DELETE handlers honored If-Match but the generated spec omitted the header and the 412 response. The test checks the entity and relation PATCH and DELETE operations together, so a conditional write that loses its documentation is caught.'
kind: test
location: internal/openapi/generator_test.go:TestGenerator_ConditionalWritesDeclareIfMatch
status: active
---
