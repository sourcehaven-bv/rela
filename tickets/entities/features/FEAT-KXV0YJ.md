---
id: FEAT-KXV0YJ
type: feature
title: 'Schema-as-graph: project rela config into a rela entity graph'
description: |-
    Project rela's own configuration (schema.yaml, data-entry.yaml) into a rela entity graph, so that a schema is browsable, queryable, analyzable and editable with rela's own tools. Entity types, relation types, properties, custom types, forms, lists, kanbans, views, fields and navigation entries become entities; their structural links become relations. A meta-schema describes what a schema is.

    The payoff is not the editor: it is that config becomes DATA, so everything rela already does to data (analysis, tracing, validation, the SPA, the API, versioning) applies to config for free. Cross-file edges (a form field -> the schema property it binds) make "what breaks if I change this?" a graph query that neither YAML file can answer today.

    Validated by a throwaway spike (2026-08-18, see .ignored/schemaspike/FINDINGS.md): 2171-line schema.yaml + 1485-line data-entry.yaml projected to 530 entities / 781 relations, browsable and editable in the stock SPA, with a verified round-trip producing a 1-line diff. The spike needed NO new store and NO new metamodel loader.

    Longer-term this is the read/write path for config-in-Postgres, so tenants can build their own apps from the web.
status: proposed
---
