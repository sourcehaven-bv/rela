# Views - Declarative Context Generation

Views provide a declarative way to generate complete context from your rela graph by defining traversal patterns, filters, and derived collections.

## Overview

Instead of writing custom scripts to traverse relationships and gather related entities, you can define views in `views.yaml` that declaratively specify:

- **Entry point** - The entity type to start from
- **Traversal rules** - How to follow relationships (including recursive)
- **Filters** - Property-based filtering of collected entities
- **Derived collections** - Group/filter operations on collected data
- **Relation exports** - Export specific relation types as standalone records
- **Output options** - Content inclusion, relation title resolution

## Quick Start

### 1. Create views.yaml

Create a `views.yaml` file in your project root:

```yaml
views:
  document_context:
    description: "Complete context for a document"

    entry:
      type: document
      parameter: doc_id

    output:
      include_content: true
      resolve_relation_titles: true

    traverse:
      - from: entry
        follow: contains
        collect_as: sections
```

### 2. Execute a View

```bash
rela view document_context DOC-001 -o yaml
```

## View Definition Structure

### Entry Point

Specifies the starting entity type:

```yaml
entry:
  type: document        # Entity type
  parameter: doc_id     # Parameter name (documentation only)
```

### Output Options

```yaml
output:
  include_content: true           # Include entity/relation markdown content
  resolve_relation_titles: true   # Resolve IDs to {id, title} objects
  include_entry: true              # Include entry entity in output (default: true)
```

### Traverse Rules

Define how to follow relationships:

```yaml
traverse:
  # Follow outgoing relations
  - from: entry
    follow: contains
    collect_as: sections

  # Follow incoming relations (reverse)
  - from: sections
    follow_incoming: partOf
    collect_as: parent_documents

  # Recursive traversal
  - from: components
    follow: dependsOn
    recursive: true
    max_depth: 5
    collect_as: dependencies

  # Multiple target collections
  - from: requirements
    follow: addresses
    collect_as: [decisions, adrs]

  # Wildcard source (all collected entities)
  - from: "*"
    follow: hasTag
    collect_as: tags

  # With property filter
  - from: requirements
    follow: addresses
    where: "status=accepted"
    collect_as: accepted_decisions
```

**Traverse Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `from` | string or list | Source collection(s), `"entry"` for entry entity, `"*"` for all |
| `follow` | string | Outgoing relation type to follow |
| `follow_incoming` | string | Incoming relation type (reverse direction) |
| `collect_as` | string or list | Target collection name(s) |
| `recursive` | bool | Follow relation transitively |
| `max_depth` | int | Maximum recursion depth (default: 10) |
| `where` | string | Property filter expression |

**Type-Based Collection Filtering:**

When `collect_as` specifies multiple collection names (e.g., `[functions, usecases, scenarios]`), entities are automatically filtered by type:
- Collection name matches entity type (singular or plural)
- `functions` collection only gets entities of type `function`
- `usecases` collection only gets entities of type `usecase`

This prevents mixed entity types in collections. For generic collection names (not matching any entity type), all entities are included.

**Multi-Pass Traversal:**

The view engine runs traverse rules in multiple passes (up to 10) until no new entities are found. This ensures that entities reachable via indirect paths are discovered, even if intermediate collections aren't fully populated on the first pass.

### Filters

Apply property-based filters to collections:

```yaml
filters:
  requirements:
    # Match any of these conditions
    match_any:
      - via_traversal: true           # Reached via traverse rules
      - id_prefix: ["REQ-", "LRZA-"]  # ID starts with prefix
      - where: "priority=high"        # Property expression

  components:
    # Single condition
    where: "status=active"

  # Expand mode: add entities from graph matching criteria
  requirements_by_prefix:
    expand: true                      # Query graph for matching entities
    id_prefix: ["LRZA-", "GF-"]       # Find all entities with these prefixes
```

**Filter Options:**

| Field | Type | Description |
|-------|------|-------------|
| `via_traversal` | bool | Include entities reached via traverse rules |
| `id_prefix` | []string | Match entities with ID starting with prefix |
| `where` | string | Property filter expression |
| `match_any` | []Filter | Match any of the sub-filters (OR logic) |
| `expand` | bool | **NEW:** Query graph for entities matching criteria, not just filter existing collection |

**Filter Operators:**
- `=` - Equal
- `!=` - Not equal
- `<` - Less than
- `<=` - Less than or equal
- `>` - Greater than
- `>=` - Greater than or equal
- `=~` - Regex match

**Expand Mode:**

By default, filters only filter entities already in a collection. With `expand: true`, the filter queries the entire graph and adds matching entities to the collection:

```yaml
filters:
  requirements:
    expand: true
    id_prefix: ["LRZA-", "GF-"]
    where: "status=accepted"
```

This is useful for including entities based on naming conventions or properties rather than graph connectivity.

### Derived Collections

Create computed collections from existing ones:

```yaml
derived:
  # Group by property
  components_by_type:
    source: components
    group_by: "properties.component_type"

  # Filter subset
  high_priority_requirements:
    source: requirements
    where: "priority=high"

  # Combine operations
  active_components_by_domain:
    source: components
    where: "status=active"
    group_by: "properties.domain"
```

### Relation Exports

Export relations as standalone records:

```yaml
relation_exports:
  - types: [mapsTo, transforms]
    between: [dataobject, dataobject]
    collect_as: data_mappings

  - types: [implements, realizes]
    collect_as: implementation_links
```

## Output Structure

Views generate structured output with three sections:

```yaml
# The entry entity (if include_entry: true)
entry:
  id: DOC-001
  type: document
  properties:
    title: "Document Title"
  content: "..."
  relations:
    outgoing:
      contains:
        - { id: SEC-001, title: "Section 1" }

# Collected entities organized by collection name
collections:
  sections:
    - id: SEC-001
      type: section
      properties:
        title: "Section 1"
      content: "..."

  # Grouped collections
  components_by_type:
    backend:
      - { id: COMP-001, ... }
    frontend:
      - { id: COMP-002, ... }

# Exported relations
relations:
  data_mappings:
    - from: DO-001
      to: DO-002
      type: mapsTo
      content: "Mapping description..."
```

## Examples

### Document Publishing Context

Generate complete context for publishing a document:

```yaml
views:
  document_publish:
    description: "Complete context for document publishing"

    entry:
      type: document
      parameter: doc_id

    output:
      include_content: true
      resolve_relation_titles: true

    traverse:
      - from: entry
        follow: contains
        collect_as: sections

      - from: sections
        follow: describes
        collect_as: components

      - from: components
        follow: dependsOn
        recursive: true
        max_depth: 5
        collect_as: dependencies

      - from: "*"
        follow: hasTag
        collect_as: tags

    derived:
      components_by_type:
        source: components
        group_by: "properties.type"
```

**Usage:**
```bash
rela view document_publish DOC-001 -o yaml > context.yaml
```

### Requirements Traceability

Trace a requirement through the architecture:

```yaml
views:
  requirement_trace:
    description: "Full traceability for a requirement"

    entry:
      type: requirement
      parameter: req_id

    output:
      include_content: true
      resolve_relation_titles: true

    traverse:
      - from: entry
        follow_incoming: addresses
        collect_as: decisions

      - from: decisions
        follow_incoming: implements
        collect_as: solutions

      - from: solutions
        follow_incoming: realizes
        collect_as: components

    filters:
      decisions:
        where: "status=accepted"
```

### Component Dependencies

Get all transitive dependencies for a component:

```yaml
views:
  component_dependencies:
    description: "All dependencies for a component"

    entry:
      type: component
      parameter: comp_id

    traverse:
      - from: entry
        follow: dependsOn
        recursive: true
        max_depth: 10
        collect_as: dependencies
```

## Performance

Views execute efficiently through:

- **Single graph traversal** - One pass through the graph, no N+1 queries
- **In-memory operations** - All data loaded once from cache
- **Lazy evaluation** - Only traverse paths specified in rules

A typical view that would require 100+ subprocess calls in a script executes in milliseconds.

## Validation

Views are validated against your metamodel when executed:

- Entity types must exist
- Relation types must exist
- Property references must be valid
- Traverse rules must be well-formed

Validation errors provide clear feedback:

```
view document_publish: traverse[0]: unknown relation type: unknownRelation
```

## Best Practices

1. **Start simple** - Begin with basic traversal, add filters/derived later
2. **Use meaningful names** - Collection names should describe their contents
3. **Limit recursion depth** - Set reasonable `max_depth` to prevent cycles
4. **Filter early** - Apply `where` clauses in traverse rules when possible
5. **Group logically** - Use derived collections to organize output
6. **Document views** - Add descriptions to help others understand intent

## Future Enhancements

The following features are planned for future releases:

- **Embed operations** - Inline related entities in derived collections
- **Advanced filters** - Complex boolean expressions
- **View composition** - Reference other views as building blocks
- **Computed properties** - Calculate derived values in output

## Troubleshooting

**View not found:**
```
Error: view not found: my_view
```
→ Check that the view name matches exactly in `views.yaml`

**Entry entity not found:**
```
Error: entry entity not found: DOC-999
```
→ Verify the entity ID exists in your project

**Validation error:**
```
Error: view validation failed: entry.type: unknown entity type: doc
```
→ Check that entity types match your metamodel definitions

**Empty collections:**
If expected entities don't appear:
- Verify traverse rules use correct relation types
- Check direction (`follow` vs `follow_incoming`)
- Ensure entities are connected by the specified relations
- Add verbose output: `rela view my_view ID -v`

## See Also

- [Filter Expressions](./filters.md) - Property filter syntax
- [Metamodel](./metamodel.md) - Entity and relation definitions
- [Export Command](./export.md) - Alternative export options
