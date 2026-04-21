---
id: schema-visualization
type: concept
title: Metamodel Schema Visualization
summary: Rendering the metamodel itself (entity types and relation types) as a Graphviz DOT diagram via `rela schema --graphviz`.
description: Distinct from `rela graph` (which renders actual entities/relations). `rela schema --graphviz` produces a DOT document describing entity types as nodes and relation types as edges. Used for architecture reviews and documentation. Current output becomes unreadable for schemas with polymorphic relations (many-target `to:` lists) and for entity types that act as structural catch-alls (e.g. a `referentie` type that links to every other type).
package: internal/cli
layer: cli
status: stable
---
