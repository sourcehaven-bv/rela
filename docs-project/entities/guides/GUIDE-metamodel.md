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
    id_prefix: REQ-
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
    id_prefix: CTL-
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

Define reusable types that can be used in entity properties. Custom types support
enum values, regex validations, or both.

### Enum Types

Define allowed values for a property:

```yaml
types:
  status:
    values: [draft, proposed, accepted, deprecated, rejected, retired]
    default: draft

  priority:
    values: [critical, high, medium, low]
```

### Regex Validations

Define validation patterns with user-friendly error messages. Multiple patterns
can be combined—all must pass for a value to be valid:

```yaml
types:
  semver:
    description: "Semantic version number"
    validations:
      - pattern: '^\d+\.\d+\.\d+$'
        error: "Must be valid semver (e.g., 1.2.3)"

  rrule:
    description: "iCal recurrence rule (RFC 5545)"
    validations:
      - pattern: "^FREQ=(YEARLY|MONTHLY|WEEKLY|DAILY)"
        error: "Must start with valid FREQ"
      - pattern: "^(?!.*COUNT=.*UNTIL=)"
        error: "Cannot specify both COUNT and UNTIL"

  email:
    validations:
      - pattern: "^[^@]+@[^@]+\\.[^@]+$"
        error: "Must be a valid email address"
```

Each validation requires:

| Field     | Description                                        |
| --------- | -------------------------------------------------- |
| `pattern` | Regex pattern that values must match               |
| `error`   | User-friendly error message shown when validation fails |

**Benefits of multiple simple patterns vs one complex regex:**

- Each pattern has its own clear error message
- Users see exactly which validation failed
- Patterns are easier to write and maintain
- No mega-regex with opaque errors

### Empty Values

- **Enum types**: Empty string is not a valid value (fails validation)
- **Regex-only types**: Empty strings skip validation (let `required` handle it)
- **List properties**: Each item in the list is validated independently

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

| Field          | Description                                               |
| -------------- | --------------------------------------------------------- |
| `label`        | Display name                                              |
| `label_plural` | Plural display name (defaults to label + "s")             |
| `description`  | Documentation explaining intent and usage (optional)      |
| `aliases`      | Alternative names for CLI (e.g., `req` for `requirement`) |
| `id_type`      | `short` (default), `sequential`, or `manual` - controls ID generation |
| `id_prefix`    | Single ID prefix (e.g., `REQ-`)                           |
| `id_prefixes`  | Multiple ID prefixes (e.g., `["DEC-", "ADR-"]`)           |
| `properties`   | Property definitions                                      |
| `default_sort` | Default sort order for list views                         |
| `color`        | Fill color for graph visualizations (hex or named)        |
| `border_color` | Border color for graph visualizations                     |
| `display_property` | Property whose value names the entity. See [Display name](#display-name) below. |

### Display name

Every entity type has a *primary property* — the property whose value
is the entity's display name. When unset, rela picks one
automatically: it checks `title`, `name`, `label` in that order (when
each is a required string property), then falls back to any required
string property (alphabetical), then to the entity ID. That works for
English schemas but is brittle for non-English ones — the priority
list never matches Dutch `naam` or `titel`, so the fallback runs, and
the choice silently flips if a second required string property is
added later.

Set `display_property` explicitly to make the choice load-bearing:

```yaml
entities:
  applicatie:
    label: Applicatie
    display_property: naam
    properties:
      naam:
        type: string
        required: true
```

**Allowed types.** The named property must be `string`, `integer`,
`boolean`, or `enum` (custom enum-like types are accepted). `date`,
`file`, `rrule`, and list-typed (`list: true`) properties are
rejected at metamodel-load time — their default rendering produces
strings nobody designed as a display name (e.g. `"2026-04-25 00:00:00
+0000 UTC"`, `"[a b c]"`).

**Runtime behavior.** Non-string values (integers, booleans, enum
values) are stringified via `fmt.Sprintf("%v", val)`. The display
falls back to the entity ID when the value is empty, missing, or
`nil`.

**Validation.** A typo, whitespace mistake, list-typed reference, or
disallowed type fails metamodel-load with a diagnostic naming the
entity, the offending value, and the available properties.

How the data-entry app surfaces the display name across lists, cards,
breadcrumbs, and related-entity links is documented in
[GUIDE-data-entry.md → Display names](data-entry.md#display-names).

### ID Types

Entity IDs can be auto-generated or manually specified:

| Type         | Description                              | Example IDs                     |
| ------------ | ---------------------------------------- | ------------------------------- |
| `short`      | Random base36 IDs (default)              | `REQ-a3f8`, `REQ-k2m9`          |
| `sequential` | Auto-incremented numeric IDs             | `REQ-001`, `REQ-002`, `DEC-003` |
| `manual`     | Manually specified string IDs            | `auth-module`, `user-service`   |

**Short IDs** (default):

- Automatically generated random base36 strings
- Format: `PREFIX-XXXX` (e.g., `REQ-a3f8`)
- Compact and collision-resistant
- Excluded from gap analysis (no sequence to track)

**Sequential IDs**:

- Auto-incremented numeric suffix
- Format: `PREFIX-NNN` (e.g., `REQ-001`)
- Gap analysis detects missing numbers in sequences

**Manual IDs**:

- Require `--id` flag when creating entities
- No automatic generation
- Excluded from gap analysis

```yaml
entities:
  # Short IDs (default behavior)
  requirement:
    label: Requirement
    id_prefix: REQ-
    # id_type: short  # This is the default

  # Sequential IDs for numbered tracking
  decision:
    label: Decision
    id_prefix: ADR-
    id_type: sequential

  # Manual IDs for components/modules
  component:
    label: Component
    id_type: manual
    # id_prefix not needed for manual IDs
    properties:
      name:
        type: string
        required: true
```

Creating entities:

```bash
# Short ID (default, auto-generated)
rela create requirement -t "User authentication"
# Creates REQ-a3f8

# Sequential ID (auto-incremented)
rela create decision -t "Use PostgreSQL for persistence"
# Creates ADR-001

# Manual ID (requires --id)
rela create component --id auth-service -t "Authentication Service"
# Creates auth-service
```

### Entity Descriptions

Add a `description` field to document the intent and usage of an entity type. Descriptions
support markdown and are surfaced in the data-entry UI via help modals:

```yaml
entities:
  decision:
    label: Decision
    description: |
      A decision records an important architectural choice and its rationale.

      Use decisions when:
      - Making technology choices (frameworks, databases, etc.)
      - Defining patterns or conventions
      - Resolving requirement conflicts

      Each decision should address one or more requirements.
    properties:
      # ...
```

In the data-entry UI, a help icon (?) appears next to the entity form title. Clicking it
opens a modal showing the entity description, all properties with their descriptions, and
available relations with cardinality constraints.

### Entity Styling

Customize how entity types appear in graph visualizations with `color` and `border_color`:

```yaml
entities:
  risk:
    label: Risk
    id_prefix: RISK-
    color: "#FFEBEE"         # Light red fill
    border_color: "#C62828"  # Dark red border
    properties:
      # ...

  control:
    label: Control
    id_prefix: CTL-
    color: "#E8F5E9"         # Light green fill
    border_color: "#2E7D32"  # Dark green border
    properties:
      # ...
```

Colors can be specified as:

- Hex codes: `#FF5722`, `#4CAF50`
- Named colors: `red`, `green`, `lightblue`

These colors are used in:

- `rela graph` DOT output
- `rela schema --graphviz` visualization
- Data-entry graph views

### Example Entity Type

```yaml
entities:
  requirement:
    label: Requirement
    aliases: [req]
    id_prefix: REQ-
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
| `file`     | File attachment (stored in `.rela/attachments/`) | N/A                          |
| `<custom>` | Reference to a type defined in `types:` | `=`, `!=`                           |

### Property Options

| Option           | Description                                          |
| ---------------- | ---------------------------------------------------- |
| `required: true` | Property must be provided                            |
| `default`        | Default value for the property                       |
| `format`         | Date format (Go layout string, e.g., `2006-01-02`)   |
| `description`    | Documentation for the property                       |
| `list: true`     | Allow multiple values (multi-select for enum types)  |

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

  # File attachment
  screenshot:
    type: file
    description: "Screenshot of the issue"

  # Multi-select enum (list: true)
  tags:
    type: enum
    values: [frontend, backend, api, database, security]
    list: true  # Allows selecting multiple values
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

The inverse ID is also the key under which incoming edges are grouped in the data-entry
API's `GET /api/v1/{plural}/{id}/relations` response, and it's what surfaces in the help
modal's "Incoming relations" section. See [data-entry.md → Reverse Relations](data-entry.md#reverse-incoming-relations)
for how form widgets and list columns opt into reverse direction with `direction: incoming`.

#### Inverse name uniqueness

Inverse names must be globally unique across the metamodel. Two failure modes
are rejected at load time:

- **`inverse_name_collision`** — two relations declare the same `inverse:` ID.
  rela cannot tell which canonical relation an inverse-keyed lookup refers to,
  so this is treated as a structural error. Example:

  ```yaml
  relations:
    blocks:
      inverse: blockedBy
    prevents:
      inverse: blockedBy   # rejected: collides with `blocks`
  ```

- **`inverse_shadows_canonical`** — a relation declares `inverse: X` where `X`
  is also the name of a separate canonical relation. The metamodel author
  most likely didn't mean for `X` to refer to two different relation sets at
  once. Example:

  ```yaml
  relations:
    r1:
      inverse: r2
    r2:                    # rejected: shadows the inverse of `r1`
      from: [...]
      to: [...]
  ```

**Exception:** symmetric relations are allowed to be their own inverse:

```yaml
relations:
  related-to:
    symmetric: true
    inverse: related-to    # OK — symmetric self-inverse
```

Use the symmetric form when the relation has no preferred direction (e.g. "is
related to" reads the same from either side).

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
    id_prefix: REQ-
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
    id_prefixes: ["DEC-", "ADR-"]
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
    id_prefix: SOL-
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
    id_prefixes: ["COMP-", "AC-", "TC-"]
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
    id_prefix: RISK-
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
    id_prefix: STK-
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
    id_prefixes: ["REQ-", "FR-", "NFR-"]  # Functional and non-functional
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

### Content Validation

In addition to property-based conditions, validation rules can check markdown content structure
using the `content` field. This validates the presence of required headers in entity markdown files.

```yaml
validations:
  - name: adr-structure
    description: "ADRs must have Context and Decision headers"
    entity_type: decision
    when:
      - "status=accepted"
    content:
      required-headers:
        - "## Context"
        - "## Decision"
```

#### Required Headers

The `required-headers` field accepts a list of header checks. Each check can be:

1. **Exact match** (string): The header must match exactly, including the `#` prefix

   ```yaml
   required-headers:
     - "## Context"        # Requires exactly "## Context"
     - "### Details"       # Requires exactly "### Details"
   ```

2. **Pattern match** (regex): Use the `pattern:` prefix for flexible matching

   ```yaml
   required-headers:
     - pattern: "## (Alternative|Alternatives)"  # Matches either spelling
     - pattern: "## .+ Analysis"                 # Matches any "## X Analysis" header
   ```

#### Content Validation Example

```yaml
validations:
  # ADRs must follow the standard structure
  - name: adr-required-sections
    description: "Accepted ADRs must have Context, Decision, and Consequences sections"
    entity_type: decision
    when:
      - "status=accepted"
    content:
      required-headers:
        - "## Context"
        - "## Decision"
        - "## Consequences"
    severity: error

  # User stories should have acceptance criteria
  - name: story-acceptance-criteria
    description: "User stories should have acceptance criteria"
    entity_type: requirement
    when:
      - "title=~^As a"
    content:
      required-headers:
        - pattern: "## (Acceptance Criteria|AC)"
    severity: warning
```

#### How Content Validation Works

1. Headers are extracted from the entity's markdown content using a proper parser
2. Headers inside code blocks (fenced or indented) are ignored
3. Each required header is checked against the extracted headers
4. If any required header is missing, the entity violates the rule

### Lua Validation

For complex validation logic that goes beyond property filters and content checks, you can use
Lua scripts. This enables cross-entity lookups, custom calculations, and sophisticated business rules.

#### Inline Lua Code

Use the `lua` field for short validation logic:

```yaml
validations:
  - name: status-required
    description: "Status must not be empty"
    entity_type: ticket
    lua: |
      local status = entity.properties.status
      if status == nil or status == "" then
        return { message = "Status is required" }
      end
      return nil
    severity: error
```

#### External Lua Scripts

For longer scripts, use `lua_file` to reference a script in the `validations/` directory.
Use `lua_args` to pass parameters to the script (available as `rela.args`):

```yaml
validations:
  - name: component-coverage-high
    description: "Critical components need 90% coverage"
    entity_type: component
    when:
      - "criticality=high"
    lua_file: check-coverage.lua
    lua_args: ["90"]
    severity: error
  - name: component-coverage-standard
    description: "Components need 80% coverage"
    entity_type: component
    lua_file: check-coverage.lua
    lua_args: ["80"]
    severity: warning
```

```lua
-- validations/check-coverage.lua
-- Entity is available as a global variable
-- Arguments are available via rela.args

local min_coverage = tonumber(rela.args[1]) or 80

local coverage = entity.properties.test_coverage
if coverage == nil then
  return nil  -- No coverage data, pass
end

-- Parse percentage (e.g., "85%" -> 85)
local value = tonumber(string.match(coverage, "(%d+)"))
if value == nil then
  return nil  -- Can't parse, pass
end

if value < min_coverage then
  return { message = "Coverage is " .. value .. "%, minimum is " .. min_coverage .. "%" }
end
return nil
```

#### Entity Context

The `entity` global variable provides access to the entity being validated:

| Field | Type | Description |
| ----- | ---- | ----------- |
| `entity.id` | string | Entity ID (e.g., "REQ-001") |
| `entity.type` | string | Entity type (e.g., "requirement") |
| `entity.properties` | table | Property key-value pairs |
| `entity.content` | string | Markdown body content |

Access properties directly via `entity.properties.status` or `entity.properties["my-field"]`.

#### Cross-Entity Lookups

Lua validation scripts have read-only access to the workspace for cross-entity validation:

```lua
-- Get another entity by ID
local related = rela.get_entity("REQ-001")
if related and related.properties.status ~= "approved" then
  return { message = "Related requirement must be approved" }
end

-- List entities by type
local components = rela.list_entities("component")
for _, comp in ipairs(components) do
  -- Check each component...
end

-- Trace dependencies
local deps = rela.trace_from(entity.id, 2)
for _, step in ipairs(deps.path) do
  -- Check dependency chain...
end
return nil
```

#### Return Value Semantics

Lua scripts return `nil` to pass validation, or a table (or array of tables) to report violations:

```lua
-- Pass: return nil or nothing
return nil

-- Single violation with custom message
return { message = "Status is required" }

-- Single violation with custom severity (overrides rule default)
return { message = "Consider adding a description", severity = "warning" }

-- Multiple violations from one rule
return {
  { message = "Missing owner", severity = "warning" },
  { message = "Priority not set", severity = "error" }
}
```

Each violation table has:

| Field | Type | Description |
| ----- | ---- | ----------- |
| `message` | string | Custom error message (required) |
| `severity` | string | `"error"` or `"warning"` (optional, defaults to rule's severity) |

#### Security and Sandboxing

Lua validation runs in a sandboxed environment:

- **Read-only workspace**: Scripts cannot create, update, or delete entities
- **Execution timeout**: Scripts are terminated after 5 seconds to prevent infinite loops
- **Path restrictions**: `lua_file` scripts must be in the `validations/` directory with `.lua` extension
- **No file I/O**: Scripts cannot read or write files directly

Errors in Lua scripts (syntax errors, runtime errors, timeouts) are logged and the validation
rule is skipped ("fail open") to avoid blocking the entire validation run.

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

## Automations

Automations are trigger-action rules that execute when entities change. They enable
workflow automation, automatic property updates, and entity creation based on state
transitions.

### Automation Structure

```yaml
automations:
  - name: automation-name
    description: "Human-readable description"
    on:
      # Trigger conditions
    do:
      # Actions to perform
```

### Triggers

Automations fire based on entity changes:

| Trigger Field     | Description                                    | Example                   |
| ----------------- | ---------------------------------------------- | ------------------------- |
| `entity`          | Entity types to watch (string or list)         | `[ticket, bug]`           |
| `property`        | Property name to monitor                       | `status`                  |
| `becomes`         | Value the property changed to                  | `in-progress`             |
| `from`            | Value the property changed from                | `backlog`                 |
| `created`         | Fires when entity is created                   | `true`                    |
| `relation_created`| Fires when this relation type is created       | `implements`              |
| `relation_removed`| Fires when this relation type is removed       | `implements`              |
| `when`            | Property conditions that must match (AND)      | `["kind=enhancement"]`    |

### Conditional Triggers

Use `when` to add property conditions that must be satisfied for the automation to fire.
This uses the same filter syntax as validation rules.

```yaml
automations:
  - name: mark-enhancement-for-docs
    description: Mark enhancement tickets for documentation review
    on:
      entity: ticket
      property: status
      becomes: review
      when:
        - "kind=enhancement"
    do:
      - set: needs_docs
        value: "true"
```

Multiple conditions use AND logic (all must match):

```yaml
on:
  entity: ticket
  property: status
  becomes: review
  when:
    - "kind=enhancement"
    - "priority=high"
```

Supported operators: `=`, `!=`, `<`, `<=`, `>`, `>=`, `=~` (regex).

**Note:** Conditions are evaluated against the entity's NEW state (after the change).
For property change triggers, use `from` to filter on the old value of the changed property.

### Actions

Actions execute when triggers match:

**Set Property**:

```yaml
do:
  - set: started_at
    value: "{{today}}"
```

**Create Relation**:

```yaml
do:
  - create_relation:
      relation: implements
      to: "{{entity.parent}}"
```

**Create Entity** (with optional relation):

```yaml
do:
  - create_entity:
      type: checklist
      properties:
        title: "Planning: {{new.title}}"
        status: in-progress
      relation: has-planning
      if_exists: skip
```

### Template Variables

Automation values support template substitution:

| Variable          | Description                              |
| ----------------- | ---------------------------------------- |
| `{{today}}`       | Current date in ISO 8601 format          |
| `{{new.title}}`   | Property value from the changed entity   |
| `{{new.status}}`  | Any property from the changed entity     |
| `{{entity.id}}`   | Entity ID                                |
| `{{user.name}}`   | Current user's name                      |

### Example: Workflow Checklists

Automatically create workflow checklists when tickets transition through stages:

```yaml
automations:
  # Create planning checklist when ticket enters planning
  - name: ticket-planning-checklist
    description: Create planning checklist when ticket enters planning
    on:
      entity: [ticket]
      property: status
      becomes: planning
    do:
      - create_entity:
          type: planning-checklist
          properties:
            title: "Planning: {{new.title}}"
            status: in-progress
          relation: has-planning
          if_exists: skip

  # Create implementation checklist when ticket enters in-progress
  - name: ticket-implementation-checklist
    description: Create implementation checklist when ticket starts
    on:
      entity: [ticket, bug]
      property: status
      becomes: in-progress
    do:
      - create_entity:
          type: implementation-checklist
          properties:
            title: "Implementation: {{new.title}}"
            status: in-progress
          relation: has-implementation
          if_exists: skip

  # Create review checklist when ticket enters review
  - name: ticket-review-checklist
    description: Create review checklist when ticket enters review
    on:
      entity: [ticket, bug]
      property: status
      becomes: review
    do:
      - create_entity:
          type: review-checklist
          properties:
            title: "Review: {{new.title}}"
            status: in-progress
          relation: has-review
          if_exists: skip
```

### Example: Status Tracking

Track when work started and by whom:

```yaml
automations:
  - name: track-started
    description: Record when work started
    on:
      entity: [ticket, bug]
      property: status
      becomes: in-progress
    do:
      - set: started_at
        value: "{{today}}"
      - set: started_by
        value: "{{user.name}}"
```

### Automation Options

| Field      | Description                                           |
| ---------- | ----------------------------------------------------- |
| `if_exists`| Behavior when `create_entity` target exists: `skip`   |

### Best Practices

1. **Use descriptive names**: Name automations after what they accomplish
2. **Keep actions focused**: Each automation should do one logical thing
3. **Use `if_exists: skip`**: Prevent duplicate entities when re-entering states
4. **Document with description**: Explain the workflow the automation supports
