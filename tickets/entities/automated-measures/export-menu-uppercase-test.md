---
id: export-menu-uppercase-test
type: automated-measure
title: 'Test: export menu uppercases only bare format names'
description: Export items in the list more menu rendered as EXPORTEREN ALS DOCX because a scoped rule uppercased every item.
kind: test
location: frontend/src/components/entity/ExportMenu.test.ts
status: active
---

Asserts the uppercase class is on bare format names and absent from the
more-mode sentence.
