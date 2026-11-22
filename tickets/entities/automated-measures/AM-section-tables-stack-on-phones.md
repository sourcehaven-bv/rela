---
id: AM-section-tables-stack-on-phones
type: automated-measure
title: View section tables stack on phones
description: The e2e mobile spec pins that a view table section stacks instead of scrolling and that a list table starts under its header. Unit tests pin the stacked cell classes, labels and roles, the empty-cell rules, the title link class, and that primary reaches the client.
kind: test
location: e2e/tests/mobile-layout.spec.ts, frontend/src/utils/sectionTable.test.ts, frontend/src/components/entity/SectionTableRowCells.test.ts, frontend/src/components/lists/EntityList.titleLink.test.ts, internal/dataentry/sections_editable_table_test.go
status: active
---

Fails when a table section scrolls sideways on a phone, when an empty or
`primary: false` cell shows in a stacked row, when a list leaves a band above
its table, or when the title link loses its link class.
