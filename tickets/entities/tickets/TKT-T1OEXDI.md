---
id: TKT-T1OEXDI
type: ticket
title: Generate TypeScript wire types from apiwire/v1
kind: chore
priority: medium
effort: m
status: backlog
---

## Description

The TypeScript wire types in `frontend/src/api` are written by hand. Generate
them from `internal/apiwire/v1` and add a CI step that fails when the generated
file is out of date. The same source later feeds Swift and Kotlin types for
native clients.

Done when the SPA imports the generated types for every `apiwire/v1` type and
`just ci` checks freshness.

Part of FEAT-QRI3CFR (step 0.1).
