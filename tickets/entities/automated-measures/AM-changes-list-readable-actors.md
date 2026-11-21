---
id: AM-changes-list-readable-actors
type: automated-measure
title: The Changes list names actors and incoming relations readably; History needs versions
description: Go and frontend tests pin actor titles, identity labels, incoming relation labels and that History is hidden for an empty timeline.
kind: test
location: internal/dataentry/audit_titles_test.go, frontend changeText.test.ts, ChangesPanel.test.ts, useHasHistory.test.ts, EntityDetail.world.test.ts, HistoryView.origin.test.ts
status: active
---

Fails when the Changes list shows a readable person as a raw id, a webhook or
integration identity by its raw name, an incoming relation without its inverse
label, when History shows for a face with no versions, or when the history page
names the entity by type and id.
