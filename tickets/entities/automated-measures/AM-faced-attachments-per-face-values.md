---
id: AM-faced-attachments-per-face-values
type: automated-measure
title: 'Attachments on a face: upload/download/delete address the face; shared bytes survive a sibling''s replace and delete'
description: Upload, download, delete and export on ID@face succeed; after a fields:all copy, replacing or deleting the file on one face leaves the other face's file downloadable.
kind: test
location: internal/dataentry/attachment_face_test.go, internal/attachment/faced_test.go, internal/mcp/tools_attachment_test.go (TestAttachments_PerFace), e2e/tests/faces-backlog.spec.ts (BUG-CTUW2N describe)
status: active
---
