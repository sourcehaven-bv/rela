---
id: FEAT-IB6S20
type: feature
title: 'Computed properties: schema-declared derived values, stored and indexed'
summary: Schema-declared properties whose value is derived from an expression over other properties on the same entity. Never user-editable; computed on write, persisted to the store and indexed like any other property. Supports chained derivation (A -> B -> C) with load-time cycle detection.
description: |-
    Today every entity property is authored by a human or a script. This feature lets schema.yaml declare a property as *derived*: its value comes from an expression evaluated over the other properties of the same entity, rather than from user input.

    The motivating case is recurrence: an entity carries an `rrule` property plus a start date, and a computed `next_occurrence` property is derived from them. Because the computed value is stored in the store like any other property, it is automatically searchable, filterable, sortable, and usable in views and feeds -- which a render-time-only derivation would not be.

    Core properties of the design:

    - **Declared in the metamodel.** A property definition gains a way to say "this value is computed by <expression>".
    - **Never editable.** Any attempt to write a computed property through the data-entry API, CLI, MCP tools, or Lua is rejected; the data-entry form does not render an editable widget for it.
    - **Stored and indexed.** The computed value is persisted through the normal store write path (fsstore frontmatter / pgstore columns) and reaches the search index like an authored property.
    - **Chained derivation.** A computed property may read other computed properties (A -> B -> C), evaluated in dependency order.
    - **Cycles are a load error.** A circular definition fails schema load, consistent with the project's existing rule that an uncompilable condition is a load error rather than a silently dropped constraint.

    This is the "computed properties" half of the long-standing Extended Property System future-concept (IDEA-008); rollup/aggregate properties over relations (IDEA-001) are deliberately out of scope.
priority: medium
status: proposed
---
