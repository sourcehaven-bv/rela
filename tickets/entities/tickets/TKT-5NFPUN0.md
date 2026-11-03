---
id: TKT-5NFPUN0
type: ticket
title: Edit an owned row's prose on the parent page
kind: enhancement
priority: medium
status: backlog
description: Split from TKT-4Q2EIVZ; see body.
---

## Description

Let users edit an owned row's body or markdown property in place on the parent
page. Split from TKT-4Q2EIVZ after its design review (RR-7D0Q2T7).

Needs:

- a per-row content channel in `useAutoSave` with preconditions;
- content `_versions` on `ViewEntity`;
- a per-row `mentions` map from the server;
- the `guardWriteBack` path;
- tests that opening and closing a row without edits sends nothing.
