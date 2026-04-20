---
id: FEAT-F5KJ5
type: feature
title: Graph DOT export and rendering
summary: '`rela graph` exports the entity/relation graph as Graphviz DOT and optionally renders it to SVG/PNG/PDF via the external `dot` binary.'
description: 'Implemented in internal/cli/graph.go. Supports: stdout DOT, writing DOT to a file, rendering to image formats via Graphviz, rankdir TB/LR, and filtering by entity type.'
priority: medium
status: implemented
---
