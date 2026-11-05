---
id: TKT-C91W7H4
type: ticket
title: Schema namespaces and variants (load-time expansion)
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Build option C of RES-TFI5YCG: schema namespaces and variants as load-time
sugar.

- An included file may declare `namespace: <name>`. Every name it declares (entity types, relation types, inverse names, custom types, validations, automations) gets the prefix `<name>.`.
- Names in a namespaced file resolve locally first, then in the root.
- The dot is reserved: a hand-written name or `plural` must not contain one.
- `abstract: true` declares a shape that stores no entities. `extends: <abstract>` makes a variant that inherits properties, faces and settings. A variant may add and override properties but not remove them.
- A bare abstract name expands to its variants in every schema surface that takes a set of types (relation endpoints, validations, automations, comments, world overrides), and in `acl.yaml` and `data-entry.yaml`. A surface that takes exactly one type rejects an abstract name with an error that lists its variants.
- At runtime only concrete types exist. Naming an abstract type in the CLI, REST, MCP or Lua returns an error listing its variants.
- `abstract: {conversions: true}` on an abstract type generates a conversion between each pair of its variants.
- The root `namespace:` key (an unused IRI) is renamed to `iri:`, with a project-file migration.
- Included files may now declare `automations:`; before this they were silently dropped.
