<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Views - Declarative Context Generation

Views provide a declarative way to generate complete context from your rela graph by defining
traversal patterns, filters, and derived collections.

## Overview

Instead of writing custom scripts to traverse relationships and gather related entities, you can
define views in `views.yaml` that declaratively specify:

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
  type: document # Entity type
  parameter: doc_id # Parameter name (documentation only)
```

### Output Options

```yaml
output:
  include_content: true # Include entity/relation markdown content
  resolve_relation_titles: true # Resolve IDs to {id, title} objects
  include_entry: true # Include entry entity in output (default: true)
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

| Field             | Type           | Description                                                     |
| ----------------- | -------------- | --------------------------------------------------------------- |
| `from`            | string or list | Source collection(s), `"entry"` for entry entity, `"*"` for all |
| `follow`          | string         | Outgoing relation type to follow                                |
| `follow_incoming` | string         | Incoming relation type (reverse direction)                      |
| `collect_as`      | string or list | Target collection name(s)                                       |
| `recursive`       | bool           | Follow relation transitively                                    |
| `max_depth`       | int            | Maximum recursion depth (default: 10)                           |
| `where`           | string         | Property filter expression                                      |

**Type-Based Collection Filtering:**

When `collect_as` specifies multiple collection names (e.g., `[functions, usecases, scenarios]`),
entities are automatically filtered by type:

- Collection name matches entity type (singular or plural)
- `functions` collection only gets entities of type `function`
- `usecases` collection only gets entities of type `usecase`

This prevents mixed entity types in collections. For generic collection names (not matching any
entity type), all entities are included.

**Multi-Pass Traversal:**

The view engine runs traverse rules in multiple passes (up to 10) until no new entities are found.
This ensures that entities reachable via indirect paths are discovered, even if intermediate
collections aren't fully populated on the first pass.

### Filters

Apply property-based filters to collections:

```yaml
filters:
  requirements:
    # Match any of these conditions
    match_any:
      - via_traversal: true # Reached via traverse rules
      - id_prefix: ["REQ-", "LRZA-"] # ID starts with prefix
      - where: "priority=high" # Property expression

  components:
    # Single condition
    where: "status=active"

  # Expand mode: add entities from graph matching criteria
  requirements_by_prefix:
    expand: true # Query graph for matching entities
    id_prefix: ["LRZA-", "GF-"] # Find all entities with these prefixes
```

**Filter Options:**

| Field           | Type     | Description                                                                              |
| --------------- | -------- | ---------------------------------------------------------------------------------------- |
| `via_traversal` | bool     | Include entities reached via traverse rules                                              |
| `id_prefix`     | []string | Match entities with ID starting with prefix                                              |
| `where`         | string   | Property filter expression                                                               |
| `match_any`     | []Filter | Match any of the sub-filters (OR logic)                                                  |
| `expand`        | bool     | **NEW:** Query graph for entities matching criteria, not just filter existing collection |

**Filter Operators:**

- `=` - Equal
- `!=` - Not equal
- `<` - Less than
- `<=` - Less than or equal
- `>` - Greater than
- `>=` - Greater than or equal
- `=~` - Regex match

**Expand Mode:**

By default, filters only filter entities already in a collection. With `expand: true`, the filter
queries the entire graph and adds matching entities to the collection:

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

```text
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

## Dependency Analysis & CI Integration

Views can be used to determine which entities contribute to a document, and which documents are
affected by changes. This is useful for CI pipelines that build artifacts (e.g., PDFs) only when
relevant entities have changed.

### The Problem

Consider a project with 50 documents, each built from a view that traverses requirements, sections,
and components. Rebuilding all 50 PDFs on every commit is wasteful when only 2 documents were
actually affected by a change. Rela provides two commands to solve this:

- **`rela view deps`** — Lists all entity IDs (or file paths) that contribute to a view's output
- **`rela view affected`** — Given a set of changed entities, reports which document roots are affected

### Listing View Dependencies

Use `rela view deps` to list all entity IDs that a view touches:

```bash
# All entities used across all documents of this view's entry type
rela view deps document_publish

# For specific roots only
rela view deps document_publish --roots DOC-001,DOC-002

# Output file paths instead of IDs (for comparison with git diff)
rela view deps document_publish --files
```

Output is one ID (or file path) per line, sorted and deduplicated.

The `--files` flag is particularly useful because the output can be directly compared with
`git diff --name-only` to determine which documents were affected.

### Finding Affected Documents

Use `rela view affected` to find which document roots are affected by entity changes:

```bash
# By entity IDs
rela view affected document_publish --changed REQ-001,COMP-003

# By file paths
rela view affected document_publish --changed-files entities/requirement/REQ-001.md

# Pipe git diff output via stdin
git diff --name-only HEAD~1 | rela view affected document_publish --changed-files -

# Restrict to specific roots
rela view affected document_publish --changed REQ-001 --roots DOC-001,DOC-002
```

Both entity file paths and relation file paths are recognized. When a relation file changes,
both endpoint entities are treated as changed.

### How It Works

The `view affected` command works by:

1. Executing the view for each root entity (or all entities of the view's entry type)
2. Collecting all entity IDs touched during each execution
3. Checking whether any of the changed entity IDs appear in each root's dependency set
4. Outputting the root IDs where a match was found

This means that **transitive** changes are detected automatically. If a component changes and
a document includes a section that describes that component, the document is reported as affected
even though the document doesn't directly reference the component.

### CI Integration Patterns

#### Shell Script: Selective Builds

The simplest approach — a standalone script you can call from any CI system:

```bash
#!/bin/bash
# build-affected-docs.sh
# Usage: ./build-affected-docs.sh [base-ref]
set -euo pipefail

BASE_REF="${1:-HEAD~1}"
VIEW_NAME="document_publish"

# Get changed files since base ref
changed_files=$(git diff --name-only "$BASE_REF")

if [ -z "$changed_files" ]; then
  echo "No files changed"
  exit 0
fi

# Find affected document roots
affected=$(echo "$changed_files" | rela view affected "$VIEW_NAME" --changed-files -)

if [ -z "$affected" ]; then
  echo "No documents affected by changes"
  exit 0
fi

echo "Affected documents:"
echo "$affected"
echo ""

# Build only affected documents
for doc_id in $affected; do
  echo "Building $doc_id..."
  rela view "$VIEW_NAME" "$doc_id" -o yaml | build_pdf --stdin -o "output/${doc_id}.pdf"
done

echo "Done. Built $(echo "$affected" | wc -l | tr -d ' ') document(s)."
```

#### GitHub Actions

<!-- markdownlint-disable line-length -->

```yaml
name: Build Documents

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build-docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Full history for accurate diff

      - name: Install rela
        run: go install github.com/Sourcehaven-BV/rela/cmd/rela@latest

      - name: Find affected documents
        id: affected
        run: |
          if [ "${{ github.event_name }}" = "pull_request" ]; then
            BASE="${{ github.event.pull_request.base.sha }}"
          else
            BASE="HEAD~1"
          fi

          AFFECTED=$(git diff --name-only "$BASE" | rela view affected document_publish --changed-files - || true)

          if [ -z "$AFFECTED" ]; then
            echo "No documents affected"
            echo "has_affected=false" >> "$GITHUB_OUTPUT"
          else
            echo "Affected documents:"
            echo "$AFFECTED"
            echo "has_affected=true" >> "$GITHUB_OUTPUT"
            # Store as multiline output
            {
              echo "doc_ids<<EOF"
              echo "$AFFECTED"
              echo "EOF"
            } >> "$GITHUB_OUTPUT"
          fi

      - name: Build affected documents
        if: steps.affected.outputs.has_affected == 'true'
        run: |
          mkdir -p output
          echo "${{ steps.affected.outputs.doc_ids }}" | while read -r doc_id; do
            echo "Building $doc_id..."
            rela view document_publish "$doc_id" -o yaml | build_pdf --stdin -o "output/${doc_id}.pdf"
          done

      - name: Upload documents
        if: steps.affected.outputs.has_affected == 'true'
        uses: actions/upload-artifact@v4
        with:
          name: documents
          path: output/
```

<!-- markdownlint-enable line-length -->

#### GitLab CI

```yaml
build-documents:
  stage: build
  script:
    - |
      AFFECTED=$(git diff --name-only "$CI_MERGE_REQUEST_DIFF_BASE_SHA" \
        | rela view affected document_publish --changed-files - || true)

      if [ -z "$AFFECTED" ]; then
        echo "No documents affected by changes"
        exit 0
      fi

      mkdir -p output
      for doc_id in $AFFECTED; do
        echo "Building $doc_id..."
        rela view document_publish "$doc_id" -o yaml \
          | build_pdf --stdin -o "output/${doc_id}.pdf"
      done
  artifacts:
    paths:
      - output/
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
```

### Alternative: File-Based Intersection

If you prefer scripting the intersection yourself rather than using `view affected`, you can
use `view deps --files` and compare directly with `git diff`:

```bash
#!/bin/bash
# Using comm to intersect file lists
changed=$(git diff --name-only HEAD~1 | sort)
deps=$(rela view deps document_publish --roots DOC-001 --files | sort)

overlap=$(comm -12 <(echo "$changed") <(echo "$deps"))

if [ -n "$overlap" ]; then
  echo "DOC-001 is affected by changes to:"
  echo "$overlap"
fi
```

This gives you more control but requires iterating over roots manually. The `view affected`
command does this for you in a single call.

### Choosing the Right Base Reference

The base reference for `git diff` determines which changes are considered:

| Scenario | Base reference | Description |
| --- | --- | --- |
| Last commit | `HEAD~1` | Changes in the most recent commit |
| Pull request | PR base SHA | All changes in the PR branch |
| Since last release | `v1.0.0` | All changes since a tag |
| Since last build | stored SHA | Track the last successfully built commit |

For pull requests, most CI systems provide the base SHA automatically
(e.g., `github.event.pull_request.base.sha` in GitHub Actions,
`CI_MERGE_REQUEST_DIFF_BASE_SHA` in GitLab CI).

### Tips

- **Full git history**: Use `fetch-depth: 0` (GitHub Actions) or equivalent to ensure
  `git diff` can reach the base reference
- **Relation changes matter**: When a relation file changes, both endpoint entities are
  marked as changed — this catches structural changes like adding or removing links
- **Metamodel changes**: If `metamodel.yaml` or `views.yaml` changes, consider rebuilding
  all documents since the schema itself changed
- **Force rebuilds**: Pass all root IDs as `--changed` to force a full rebuild:
  `rela view affected document_publish --changed $(rela list document --ids)`

## Troubleshooting

**View not found:**

```text
Error: view not found: my_view
```

→ Check that the view name matches exactly in `views.yaml`

**Entry entity not found:**

```text
Error: entry entity not found: DOC-999
```

→ Verify the entity ID exists in your project

**Validation error:**

```text
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
