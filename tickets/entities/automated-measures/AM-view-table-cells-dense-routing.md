---
id: AM-view-table-cells-dense-routing
type: automated-measure
title: 'Test: view table cells route like list cells'
description: 'Mounts a display: table section and asserts a title renders as text, a date formatted, a boolean as Yes/No and only an enum as a badge; checks the server writes dates as ISO. Catches a table cell that badges every typed value (BUG-UI5TT6).'
kind: test
location: frontend/src/components/entity/EntityDetail.table.test.ts; internal/dataentry sections_test.go (TestPropertyToStrings)
status: active
---
