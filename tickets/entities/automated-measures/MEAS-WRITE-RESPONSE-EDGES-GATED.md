---
id: MEAS-WRITE-RESPONSE-EDGES-GATED
type: automated-measure
title: Write responses serialize only visible neighbour edges
description: Test that create, upload and restore responses omit edges to neighbours the caller cannot read, as GET does.
kind: test
location: internal/dataentry/write_handler_test.go
status: proposed
---

A test for create, upload and restore: an edge to a neighbour the caller cannot
read is absent from the response's relations map, matching GET.
