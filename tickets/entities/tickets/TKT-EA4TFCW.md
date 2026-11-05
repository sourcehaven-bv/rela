---
id: TKT-EA4TFCW
type: ticket
title: Server resolves the widget for every field
kind: refactor
priority: medium
effort: s
status: backlog
---

## Description

`ViewSectionField.widget` is only set when config overrides it; otherwise the
SPA picks a widget with `defaultWidgetFor`. Have the server set `widget` on
every field, applying the same defaults, and delete the client dispatch.

Part of FEAT-QRI3CFR (step 1.1).
