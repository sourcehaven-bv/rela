---
id: GUIDE-metamodel
type: guide
title: "Metamodel Reference"
status: published
order: 4
audience: intermediate
summary: "Configure entity types and relations"
---

The metamodel defines your project's entity types, properties, and relations.
It's stored in `metamodel.yaml` at your project root.

## Structure

```yaml
version: "1.0"
namespace: "https://example.org/ontology/architecture#"

types:
  # Custom enum types

entities:
  # Entity type definitions

relations:
  # Relation definitions
```

## Including Partial Metamodels

For larger projects, you can split your metamodel across multiple files using the
`includes:` key. This keeps each domain's definitions in a focused, manageable file.

### Syntax

```yaml
# metamodel.yaml
version: "1.0"
namespace: "https://example.org/ontology/architecture#"

includes:
  - compliance/controls.yaml
  - risk.yaml

types:
  status:
    values: [draft, proposed, accepted, deprecated]
    default: draft

entities:
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
    properties:
      title:
        type: string
        required: true
```

The `includes:` key is always a YAML list of file paths, resolved relative to the
project root (where `metamodel.yaml` lives).

### Included File Format

Each included file is a partial metamodel. It can contain any combination of
`types:`, `entities:`, `relations:`, and `validations:` — but **must not** contain
`version:` or `namespace:` (these are only allowed in the root `metamodel.yaml`).

```yaml
# compliance/controls.yaml
types:
  applicability:
    values: [applicable, not_applicable, partial]

entities:
  control:
    label: Control
    id_patterns: ["CTL-"]
    properties:
      title:
        type: string
        required: true
      applicability:
        type: applicability

relations:
  implements_control:
    label: implements
    from: [requirement]
    to: [control]
    inverse: implementedBy

validations:
  - name: controls-need-applicability
    description: "Controls must have applicability set"
    entity_type: control
    then:
      - "applicability!="
    severity: warning
```

### Nested Includes

Included files can themselves include other files:

```yaml
# compliance/controls.yaml
includes:
  - shared/audit-types.yaml

entities:
  control:
    # ...
```

Circular includes are detected and produce a clear error:

```text
circular include detected: metamodel.yaml → compliance/controls.yaml → shared/audit-types.yaml → compliance/controls.yaml
```

### Diamond Includes

If the same file is reachable from multiple include paths (a "diamond" pattern),
it is loaded only once. This is not an error.

```yaml
# metamodel.yaml
includes:
  - a.yaml    # includes shared.yaml
  - b.yaml    # also includes shared.yaml — loaded once, no conflict
```

### Conflict Handling

If the same type, entity, relation, or validation name is defined in more than
one file, loading fails with an error identifying both files:

```text
duplicate entity "control": defined in both compliance/controls.yaml and risk.yaml
```

To resolve conflicts, rename one of the definitions or move it to a shared file.

### Error Messages

| Situation | Error |
| --- | --- |
| Duplicate definition | `duplicate entity "control": defined in both a.yaml and b.yaml` |
| Circular include | `circular include detected: a.yaml → b.yaml → a.yaml` |
| File not found | `include file not found: missing.yaml (included from metamodel.yaml)` |
| Root-only field | `included file a.yaml must not contain "version" (only allowed in root metamodel.yaml)` |

## Custom Types

Define reusable enum types that can be used in entity properties:

```yaml
types:
  status:
    values: [draft, proposed, accepted, deprecated, rejected, retired]
    default: draft

  priority:
    values: [critical, high, medium, low]
```

### Reserved Type Names

The following names are reserved for built-in property types and cannot be used as custom type names:

- `string` - Free-form text
- `date` - Date values
- `integer` - Whole numbers
- `boolean` - True/false values
- `enum` - Inline enumeration (use `values:` directly in property definition)

Attempting to define a custom type with a reserved name will produce an error:

```text
cannot define custom type "string": name is reserved for built-in type
```

## Entity Types

Each entity type defines:

| Field         | Description                                               |
| ------------- | --------------------------------------------------------- |
| `label`       | Display name                                              |
| `aliases`     | Alternative names for CLI (e.g., `req` for `requirement`) |
| `id_type`     | `auto` (default) or `manual` - controls ID generation     |
| `id_patterns` | ID prefixes (e.g., `REQ-`, `ADR-`)                        |
| `properties`  | Property definitions                                      |

### ID Types

Entity IDs can be either auto-generated or manually specified:

| Type     | Description                          | Example IDs                     |
| -------- | ------------------------------------ | ------------------------------- |
| `auto`   | Auto-generated numeric IDs (default) | `REQ-001`, `REQ-002`, `DEC-003` |
| `manual` | Manually specified string IDs        | `auth-module`, `user-service`   |

**Auto IDs** (default):

- Automatically generated when creating entities
- Format: `PREFIX-NNN` (e.g., `REQ-001`)
- Gap analysis detects missing numbers in sequences

**Manual IDs**:

- Require `--id` flag when creating entities
- No automatic generation
- Excluded from gap analysis

```yaml
entities:
  # Auto IDs (default behavior)
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
    # id_type: auto  # This is the default

  # Manual IDs for components/modules
  component:
    label: Component
    id_type: manual
    id_patterns: [] # Patterns are optional for manual IDs
    properties:
      name:
        type: string
        required: true
```

Creating entities with string IDs:

```bash
# Sequential (auto-generated)
rela create requirement -t "User authentication"
# Creates REQ-001

# String (requires --id)
rela create component --id auth-service -t "Authentication Service"
# Creates auth-service
```

### Example Entity Type

```yaml
entities:
  requirement:
    label: Requirement
    aliases: [req]
    id_patterns: ["REQ-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      status:
        type: status # References custom type above
        required: true
      priority:
        type: priority
```

### Property Types

| Type       | Description                             | Filter Operators                    |
| ---------- | --------------------------------------- | ----------------------------------- |
| `string`   | Free-form text                          | `=`, `!=`, `=~` (regex), glob (`*`) |
| `date`     | Date value (ISO 8601 by default)        | `=`, `!=`, `<`, `<=`, `>`, `>=`     |
| `integer`  | Whole number                            | `=`, `!=`, `<`, `<=`, `>`, `>=`     |
| `boolean`  | True or false                           | `=`, `!=`                           |
| `enum`     | Inline enum with `values`               | `=`, `!=`                           |
| `<custom>` | Reference to a type defined in `types:` | `=`, `!=`                           |

### Property Options

| Option           | Description                                        |
| ---------------- | -------------------------------------------------- |
| `required: true` | Property must be provided                          |
| `format`         | Date format (Go layout string, e.g., `2006-01-02`) |
| `description`    | Documentation for the property                     |

### Date Formats

For `date` properties, specify the format using Go layout strings:

```yaml
properties:
  valid_until:
    type: date
    format: "2006-01-02" # YYYY-MM-DD (ISO 8601, default)
```

Common formats:

| Format   | Example      | Go Layout              |
| -------- | ------------ | ---------------------- |
| ISO 8601 | `2025-02-01` | `2006-01-02` (default) |
| European | `01/02/2025` | `02/01/2006`           |
| US       | `02/01/2025` | `01/02/2006`           |
| Long     | `1 Feb 2025` | `2 Jan 2006`           |

### Property Type Examples

```yaml
properties:
  # String - free-form text
  title:
    type: string
    required: true

  # Date with explicit format
  valid_until:
    type: date
    format: "2006-01-02"
    description: "When this evidence expires"

  # Integer
  risk_score:
    type: integer
    description: "Risk score from 1-10"

  # Boolean
  archived:
    type: boolean

  # Inline enum
  severity:
    type: enum
    values: [low, medium, high, critical]

  # Reference to custom type
  status:
    type: status
    required: true
```

## Relations

Relations define how entity types can be connected:

| Field            | Description                                        |
| ---------------- | -------------------------------------------------- |
| `label`          | Display name                                       |
| `description`    | Explanation of the relation's meaning              |
| `from`           | Source entity types (list)                         |
| `to`             | Target entity types (list)                         |
| `inverse`        | Inverse relation definition (string or object)     |
| `symmetric`      | `true` if relation is bidirectional                |
| `min_outgoing`   | Minimum outgoing relations per from-side entity    |
| `max_outgoing`   | Maximum outgoing relations per from-side entity    |
| `min_incoming`   | Minimum incoming relations per to-side entity      |
| `max_incoming`   | Maximum incoming relations per to-side entity      |

### Example Relation

```yaml
relations:
  addresses:
    label: addresses
    description: A decision addresses a requirement
    from: [decision]
    to: [requirement]
    min_outgoing: 1 # Each decision must address at least one requirement
    inverse: addressedBy # Simple form - label auto-derived as "addressed by"
```

### Inverse Relations

The `inverse` field can be specified in two forms:

**Simple form** (recommended for most cases):

```yaml
inverse: addressedBy # Label auto-derived from ID
```

The label is automatically derived by converting camelCase to space-separated lowercase:

- `addressedBy` → `addressed by`
- `implementedBy` → `implemented by`

**Expanded form** (when custom label needed):

```yaml
inverse:
  id: addressedBy
  label: "is addressed by" # Custom label
```

### Cardinality Constraints

Use cardinality to enforce rules:

```yaml
relations:
  implements:
    label: implements
    from: [solution]
    to: [decision]
    min_outgoing: 1 # Every solution must implement at least one decision
    max_incoming: 1 # Each decision can only be implemented by one solution
```

Check violations with:

```bash
rela analyze cardinality
```

### Symmetric Relations

For relations that work in both directions:

```yaml
relations:
  conflictsWith:
    label: conflicts with
    from: [requirement, decision]
    to: [requirement, decision]
    symmetric: true
```

## Default Metamodel

When you run `rela init`, this default metamodel is created:

```yaml
version: "1.0"
namespace: "https://example.org/ontology/architecture#"

types:
  status:
    values: [draft, proposed, accepted, deprecated, rejected, retired]
    default: draft

  priority:
    values: [critical, high, medium, low]

entities:
  requirement:
    label: Requirement
    aliases: [req]
    id_patterns: ["REQ-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      status:
        type: status
        required: true
      priority:
        type: priority

  decision:
    label: Decision
    aliases: [dec, adr]
    id_patterns: ["DEC-", "ADR-"]
    properties:
      title:
        type: string
        required: true
      rationale:
        type: string
      status:
        type: status
        required: true

  solution:
    label: Solution
    aliases: [sol]
    id_patterns: ["SOL-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      status:
        type: status

  component:
    label: Component
    aliases: [comp]
    id_patterns: ["COMP-", "AC-", "TC-"]
    properties:
      title:
        type: string
        required: true

relations:
  addresses:
    label: addresses
    description: A decision addresses a requirement
    from: [decision]
    to: [requirement]
    inverse: addressedBy

  implements:
    label: implements
    description: A solution implements a decision
    from: [solution]
    to: [decision]
    inverse: implementedBy

  realizes:
    label: realizes
    description: A component realizes a solution
    from: [component]
    to: [solution]
    inverse: realizedBy

  dependsOn:
    label: depends on
    from: [component, solution, decision]
    to: [component, solution, decision]
    inverse: dependencyOf
```

## Customization Examples

### Adding a Risk Entity Type

```yaml
entities:
  risk:
    label: Risk
    id_patterns: ["RISK-"]
    properties:
      title:
        type: string
        required: true
      likelihood:
        type: enum
        values: [low, medium, high, critical]
      impact:
        type: enum
        values: [low, medium, high, critical]

relations:
  mitigates:
    label: mitigates
    from: [decision, solution]
    to: [risk]
    inverse: mitigatedBy
```

### Adding a Stakeholder Type

```yaml
entities:
  stakeholder:
    label: Stakeholder
    aliases: [stk]
    id_patterns: ["STK-"]
    properties:
      name:
        type: string
        required: true
      role:
        type: string

relations:
  ownedBy:
    label: owned by
    from: [requirement, decision, component]
    to: [stakeholder]
    inverse: owns
```

### Multiple ID Patterns

Support different ID conventions in the same project:

```yaml
entities:
  requirement:
    label: Requirement
    aliases: [req]
    id_patterns: ["REQ-", "FR-", "NFR-"] # Functional and non-functional
```

## After Modifying the Metamodel

After editing `metamodel.yaml`:

```bash
# Rebuild the cache
rela sync

# Verify with
rela tui
# Press 'm' to see the updated metamodel
```

Note: Existing entities remain valid. The metamodel only affects creation and validation of new entities and relations.

## Filtering Entities

Filter entities by property values using the `--where` flag:

```bash
# Exact match
rela list control --where "status=accepted"

# Glob pattern (strings only, use * for wildcard)
rela list control --where "iso27001=A.9.*"

# Regex match (strings only)
rela list control --where "title=~access.*policy"

# Date comparison
rela list evidence --where "valid_until<2025-02-01"
rela list evidence --where "valid_until>=2025-01-01"

# Integer comparison
rela list risk --where "risk_score>=5"
rela list risk --where "risk_score<10"

# Boolean filter
rela list evidence --where "archived=false"

# Multiple filters (AND logic)
rela list control --where "status=implemented" --where "applicability=applicable"
```

### Filter Operators

| Operator | Description                 | Supported Types   |
| -------- | --------------------------- | ----------------- |
| `=`      | Equal (exact match or glob) | All types         |
| `!=`     | Not equal                   | All types         |
| `<`      | Less than                   | `date`, `integer` |
| `<=`     | Less than or equal          | `date`, `integer` |
| `>`      | Greater than                | `date`, `integer` |
| `>=`     | Greater than or equal       | `date`, `integer` |
| `=~`     | Regex match                 | `string`          |

### Error Handling

Invalid filters produce helpful error messages:

```bash
# Unknown property
rela list control --where "typo=value"
# Error: unknown property "typo" for entity type "control"

# Invalid enum value
rela list control --where "status=invalid"
# Error: invalid value "invalid" (allowed: draft, proposed, accepted, ...)

# Invalid date format
rela list evidence --where "valid_until=not-a-date"
# Error: invalid date "not-a-date" for property "valid_until" (expected format: 2006-01-02)

# Invalid operator for type
rela list control --where "status>draft"
# Error: operator ">" not supported for enum property
```

## Sorting Entities

Sort entities by property values using the `--sort` flag:

```bash
# Sort by property (ascending)
rela list control --sort iso27001

# Sort descending
rela list evidence --sort valid_until --desc

# Sort by ID (default)
rela list control --sort id
```

Sorting is type-aware:

- `string`: Lexicographic (alphabetical)
- `enum`/custom types: By the order defined in the type's `values` list (not alphabetical)
- `date`: Chronological
- `integer`: Numeric
- `boolean`: `false` before `true`

Entities with missing values for the sort property are placed at the end.

### Default Sort Order

Entity types can declare a default sort order in the metamodel. This is used when no explicit
sort is specified in a query or CLI command:

```yaml
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    default_sort:
      - property: priority
      - property: due_date
        direction: asc
    properties:
      # ...
```

Each entry in `default_sort` is a sort criterion applied in order (first entry is the primary key).
The `direction` field is optional and defaults to `"asc"`. Supported values: `"asc"` or `"desc"`.

You can sort by any property defined on the entity, plus two virtual properties:

- `id` — sorts by entity ID
- `modified` — sorts by file modification time

### Sort in Search Queries

The TUI search screen and data entry search bar support a `sort:` clause:

```text
sort:priority                     # sort by priority ascending
sort:priority:desc                # sort by priority descending
sort:id:desc                      # sort by entity ID descending
sort:modified:desc                # sort by modification time (newest first)
sort:priority:desc sort:title     # multi-sort: priority desc, then title asc
```

When no `sort:` clause is present:

1. If all results are the same entity type and that type has `default_sort`, it is used
2. Otherwise, results are sorted by ID ascending

## Custom Validation Rules

Define validation rules to enforce business constraints on your entities.
Validation rules use the same filter syntax as `--where` filters.

### Validation Rule Structure

```yaml
validations:
  - name: rule-identifier # Unique name for the rule
    description: "Human-readable description shown in output"
    entity_type: requirement # Optional: limit to specific type
    when: # Optional: IF these conditions match...
      - "status=accepted"
    then: # THEN these must be true
      - "priority!="
    severity: error # Optional: "error" or "warning" (default)
```

### How Validation Rules Work

1. **Select entities**: If `entity_type` is specified, only those entities are checked
2. **Apply when filter**: If `when` is specified, only entities satisfying ALL when conditions are subject to the rule
3. **Check then conditions**: Matched entities must satisfy ALL `then` conditions
4. **Report violations**: Entities that match `when` but don't satisfy `then` are reported

### Example Validation Rules

```yaml
validations:
  # Accepted requirements must have a priority
  - name: accepted-needs-priority
    description: "Accepted requirements must have a priority assigned"
    entity_type: requirement
    when:
      - "status=accepted"
    then:
      - "priority!="
    severity: error

  # All decisions should have a rationale (no 'when' = applies to all)
  - name: decisions-need-rationale
    description: "Decisions should have a rationale documented"
    entity_type: decision
    then:
      - "rationale!="
    severity: warning

  # High priority requirements must have a description
  - name: high-priority-needs-description
    description: "High priority requirements need detailed descriptions"
    entity_type: requirement
    when:
      - "priority=high"
    then:
      - "description!="
    severity: warning

  # ADRs should follow naming convention
  - name: adr-naming-convention
    description: "ADRs should follow the ADR-NNN naming pattern"
    entity_type: decision
    then:
      - "title=~^ADR-\\d+:"
    severity: warning
```

### Filter Operators in Validations

Validation rules support all the same operators as `--where` filters:

| Operator | Example                | Description                                       |
| -------- | ---------------------- | ------------------------------------------------- |
| `=`      | `status=accepted`      | Equals (supports glob patterns with `*`)          |
| `!=`     | `owner!=`              | Not equals (use empty value to check "has value") |
| `<`      | `risk_score<5`         | Less than (dates, integers)                       |
| `<=`     | `deadline<=2025-12-31` | Less than or equal                                |
| `>`      | `priority>low`         | Greater than                                      |
| `>=`     | `created>=2025-01-01`  | Greater than or equal                             |
| `=~`     | `title=~^ADR-\\d+`     | Regex match (strings)                             |

### Running Validations

```bash
# Run only custom validations
rela analyze validations

# Run all analyses including validations
rela analyze all
```

### Validation Output

```text
$ rela analyze validations
✗ Accepted requirements must have a priority assigned (2):
  REQ-003: User authentication
  REQ-007: Data encryption
⚠ Decisions should have a rationale documented (1):
  DEC-002: Use PostgreSQL
Found 2 errors, 1 warnings across 2 rules
```

### Severity Levels

- **error**: Critical violations that should be fixed. Displayed with ✗
- **warning**: Recommendations that may need attention. Displayed with ⚠

### Tips

1. **Start with warnings**: Begin with `severity: warning` and promote to `error` once your data is cleaned up
2. **Use specific entity types**: Narrow rules to specific types when possible for clearer error messages
3. **Combine with cardinality**: Use cardinality constraints for relation rules, validations for property rules
4. **Check for empty values**: Use `property!=` to require that a property has any value
