---
id: FEAT-M37NWU
type: feature
title: PlantUML diagram rendering in data-entry
summary: Render ```plantuml fenced code blocks as diagrams in the data-entry UI, mirroring the existing mermaid integration. Rendering is delegated to an operator-configured remote PlantUML server (empty = disabled), so no Java runtime or large WASM bundle is added.
description: 'Adds PlantUML support alongside the existing client-side mermaid rendering. Because PlantUML has no pure-browser renderer, diagrams are rendered by a remote PlantUML server whose base URL is set per-deployment in data-entry.yaml (app.plantuml_server_url). When the URL is empty/absent the feature is disabled and plantuml blocks degrade to plain fenced code. The URL is published to the SPA via the existing /api/v1/_config endpoint; the frontend deflate+base64-encodes diagram source and points an <img> at <server>/svg/<encoded>. Two surfaces: client-rendered markdown (entity/section bodies via marked) handled purely in JS, and server-rendered documents (goldmark) which need a fence-class rewrite analogous to htmlutil.ConvertMermaidBlocks.'
priority: medium
status: proposed
---
