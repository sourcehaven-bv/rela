---
id: FEAT-U07RFLX
type: feature
title: Headless rela-server (API and MCP, no web UI)
summary: rela-server builds for sqlite and postgres that serve the REST API and MCP without embedding the SPA.
description: A headless build tag drops the embedded Vue SPA and custom-app editor bundle from rela-server so API/MCP-only deployments ship and expose no web UI. Released as separate archives.
priority: medium
status: implemented
---

## Why

API/MCP-only deployments do not need the SPA, and a smaller binary that serves
no UI is easier to reason about. See the implementing ticket for scope.
