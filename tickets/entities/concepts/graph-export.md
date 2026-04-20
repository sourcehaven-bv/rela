---
id: graph-export
type: concept
title: Graph DOT Export
summary: Rendering the entity graph to Graphviz DOT for visualization, optionally piped through `dot` for SVG/PNG/PDF.
description: The `rela graph` command serializes nodes (grouped by entity type into DOT subgraph clusters) and edges (relations) into a Graphviz DOT document. With `-f <format>` it invokes the external `dot` binary to render to SVG/PNG/PDF. DOT has specific lexical rules — unquoted identifiers must match [_A-Za-z][_A-Za-z0-9]* — so any component of the DOT produced from user-defined metamodel identifiers must either quote or sanitize those strings.
package: internal/cli
layer: cli
status: stable
---
