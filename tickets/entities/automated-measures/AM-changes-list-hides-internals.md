---
id: AM-changes-list-hides-internals
type: automated-measure
title: The Changes list leaves out internal records and checks its gate
description: Go tests pin which audit records the Changes list leaves out and that titles are whole; a frontend test pins the header_buttons.history gate.
kind: test
location: internal/visibility/auditchanges_test.go, internal/dataentry/audit_titles_test.go, frontend ChangesPanel.test.ts
status: active
---

Fails when the Changes list serves a record about an internal relation, a
renumber record, an update of only hidden properties, a partly hidden title, or
shows for a principal without the history permission.
