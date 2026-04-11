<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# CLI Reference

Complete reference for all rela commands.

## Global Options

These options work with any command:

| Option          | Description                                                          |
| --------------- | -------------------------------------------------------------------- |
| `-p, --project` | Project directory (default: auto-detect from cwd, or `RELA_PROJECT`) |
| `-o, --output`  | Output format: `table` (default) or `json`                           |
| `-v, --verbose` | Enable verbose output                                                |
| `-q, --quiet`   | Suppress non-essential output                                        |
| `-h, --help`    | Show help for any command                                            |

## Environment Variables

| Variable       | Description                                                      |
| -------------- | ---------------------------------------------------------------- |
| `RELA_PROJECT` | Default project directory when `--project` flag is not specified |

## Project Configuration

Project-level settings are stored in `.rela/config.yaml`. This file is optional; defaults are used
when it doesn't exist.

```yaml
formatting:
  line_width: 80  # Maximum line width for paragraph wrapping (default: 80)
```

| Setting                  | Description                                      | Default |
| ------------------------ | ------------------------------------------------ | ------- |
| `formatting.line_width`  | Maximum line width for paragraph wrapping        | 80      |

## Commands

### rela init

Initialize a new rela project.

```bash
rela init
rela init -p /path/to/project
```

Creates:

- `metamodel.yaml` - Default configuration
- `entities/` - Entity storage directory
- `relations/` - Relation storage directory
- `.rela/` - Cache directory (added to `.gitignore` if present)

---

### rela create

Create a new entity.

```bash
rela create <type> [flags]
```

**Arguments:**

- `type` - Entity type (e.g., `requirement`, `decision`, `solution`, `component`)

**Flags:**

| Flag              | Description                                                                    |
| ----------------- | ------------------------------------------------------------------------------ |
| `-t, --title`     | Entity title (required)                                                        |
| `-s, --status`    | Entity status (default: `draft`)                                               |
| `-p, --priority`  | Entity priority                                                                |
| `--id`            | Custom entity ID (required for string ID types, auto-generated for sequential) |
| `-P, --property`  | Set a property (format: key=value, can be repeated)                            |
| `-b, --body`      | Markdown body content for the entity                                           |
| `-B, --body-file` | Read body content from file (use `-` for stdin)                                |

**Templates:**

If a template exists at `templates/entities/<type>.md`, its frontmatter values are used as defaults
and its content is applied to the new entity. CLI flags override template defaults.
See [rela template](#rela-template) for creating templates.

**ID Types:**

Entity types can have either sequential or string IDs (configured in `metamodel.yaml`):

- **Sequential** (default): IDs are auto-generated (`REQ-001`, `REQ-002`, etc.)
- **String**: IDs must be provided with `--id` flag

```bash
# Sequential ID type (auto-generated)
rela create requirement --title "System must scale to 1000 users"
# Creates REQ-001

# String ID type (requires --id)
rela create component --id auth-service --title "Authentication Service"
# Creates auth-service
```

**Examples:**

```bash
# Create with auto-generated ID (sequential types)
rela create requirement --title "System must scale to 1000 users"

# Use type alias
rela create req -t "Short form works too"

# With status and priority
rela create requirement --title "Security audit" --status proposed --priority high

# With custom ID (works for both sequential and string types)
rela create requirement --title "Custom ID" --id REQ-CUSTOM-001

# String ID type (--id is required)
rela create component --id user-service --title "User Service"

# With custom properties
rela create control -t "Access Control" -P "iso27001=A.5.15" -P "owner=Security Team"

# With body content
rela create requirement -t "Auth feature" --body "## Description\n\nUser authentication."
```

---

### rela list

List entities with optional filtering.

```bash
rela list [type] [flags]
```

**Arguments:**

- `type` - Entity type to filter by (optional, shows all if omitted)

**Flags:**

| Flag      | Description                                      |
| --------- | ------------------------------------------------ |
| `--where` | Filter by property (repeatable for AND logic)    |
| `--sort`  | Sort by property (or `id`, `modified`)           |
| `--desc`  | Sort descending                                  |

**Filter Operators:**

The `--where` flag supports multiple comparison operators:

| Operator | Description                        | Example                            |
| -------- | ---------------------------------- | ---------------------------------- |
| `=`      | Exact match (supports `*` glob)    | `--where "status=draft"`           |
| `!=`     | Not equal                          | `--where "status!=done"`           |
| `<`      | Less than (date/integer)           | `--where "due_date<2025-03-01"`    |
| `<=`     | Less than or equal                 | `--where "priority<=2"`            |
| `>`      | Greater than                       | `--where "risk_score>5"`           |
| `>=`     | Greater than or equal              | `--where "valid_until>=2025-01-01"`|
| `=~`     | Regex match                        | `--where "title=~^Auth"`           |
| `~`      | Fuzzy match                        | `--where "title~authentcation"`    |

**Glob Patterns:**

Use `*` in equality matches for wildcard matching:

```bash
rela list control --where "iso27001=A.9.*"    # Matches A.9.1, A.9.2.1, etc.
rela list --where "title=User*"               # Titles starting with "User"
```

**Fuzzy Matching:**

The `~` operator performs fuzzy (typo-tolerant) matching:

```bash
rela list --where "title~authentcation"       # Finds "authentication" despite typo
```

**Examples:**

```bash
# List all entities
rela list

# List by type (plural or singular)
rela list requirements
rela list requirement
rela list req

# Filter by property value
rela list control --where "status=implemented"

# Multiple filters (AND logic)
rela list control --where "status=implemented" --where "applicability=applicable"

# Date comparison
rela list evidence --where "valid_until<2025-02-01"

# Integer comparison
rela list risk --where "risk_score>=5"

# Sort by property
rela list control --sort iso27001
rela list evidence --sort valid_until --desc

# JSON output
rela list -o json
```

---

### rela show

Display detailed information about an entity.

```bash
rela show <id>
```

**Arguments:**

- `id` - Entity ID to show

Shows the entity's properties plus all incoming and outgoing relations.

**Examples:**

```bash
rela show REQ-001
rela show DEC-042 -o json
```

---

### rela update

Update an entity's properties.

```bash
rela update <id> [flags]
```

**Arguments:**

- `id` - Entity ID to update

**Flags:**

| Flag                | Description     |
| ------------------- | --------------- |
| `-t, --title`       | New title       |
| `-s, --status`      | New status      |
| `-p, --priority`    | New priority    |
| `-d, --description` | New description |

At least one flag is required.

**Examples:**

```bash
# Update status
rela update REQ-001 --status accepted

# Update multiple fields
rela update DEC-042 --title "Revised title" --status proposed

# Update description
rela update SOL-001 --description "Detailed implementation notes"
```

---

### rela delete

Delete an entity.

```bash
rela delete <id> [flags]
```

**Arguments:**

- `id` - Entity ID to delete

**Flags:**

| Flag          | Description               |
| ------------- | ------------------------- |
| `-f, --force` | Skip confirmation prompt  |
| `--cascade`   | Also delete related links |

Without `--cascade`, deletion fails if the entity has relations.

**Examples:**

```bash
# Interactive confirmation
rela delete REQ-001

# Skip confirmation
rela delete REQ-001 --force

# Delete entity and all its relations
rela delete REQ-001 --cascade
```

---

### rela attach

Attach file(s) to an entity.

```bash
rela attach <entity-id> <file>... [flags]
```

Files are stored in a content-addressable store using SHA-256 hashes.
Duplicate files are automatically deduplicated.

**Arguments:**

- `entity-id` - Target entity ID
- `file...` - One or more files to attach (supports glob patterns)

**Flags:**

| Flag              | Description                              |
| ----------------- | ---------------------------------------- |
| `-P, --property`  | Property to attach file(s) to            |

If `--property` is not specified, uses the first `file`-type property defined for the entity type.

**Examples:**

```bash
rela attach BUG-042 screenshot.png
rela attach BUG-042 screenshot.png --property screenshot
rela attach DEC-007 *.pdf --property supporting-docs
rela attach REQ-001 diagram.png spec.pdf
```

---

### rela attachments

List all file attachments for an entity.

```bash
rela attachments <entity-id>
```

Shows the property name, file path, original filename, and size for each attachment.

**Arguments:**

- `entity-id` - Entity ID to list attachments for

**Examples:**

```bash
rela attachments BUG-042
rela attachments DEC-007
```

---

### rela detach

Remove an attachment reference from an entity.

```bash
rela detach <entity-id> <property> [hash-prefix]
```

This removes the reference from the entity's property but does NOT delete the
actual file. Use `rela gc --attachments` to clean up unreferenced files.

**Arguments:**

- `entity-id` - Entity ID
- `property` - Property name containing the attachment
- `hash-prefix` - Optional: first characters of hash to identify specific attachment

If the property contains multiple attachments, provide a hash prefix to specify
which one to remove. Without a prefix, removes all attachments from the property.

**Examples:**

```bash
rela detach BUG-042 screenshot
rela detach DEC-007 supporting-docs ab3f
```

---

### rela link

Create a relation between two entities.

```bash
rela link <from> <relation> <to>
```

**Arguments:**

- `from` - Source entity ID
- `relation` - Relation type name
- `to` - Target entity ID

Both entities must exist. The relation type is validated against the metamodel.

**Templates:**

If a template exists at `templates/relations/<type>.md`, its frontmatter values are used as defaults
for relation properties. See [rela template](#rela-template) for creating templates.

**Examples:**

```bash
rela link DEC-001 addresses REQ-001
rela link SOL-001 implements DEC-001
rela link COMP-001 realizes SOL-001
rela link COMP-001 dependsOn COMP-002
```

---

### rela unlink

Remove a relation between entities.

```bash
rela unlink <from> <relation> <to>
```

**Arguments:**

- `from` - Source entity ID
- `relation` - Relation type name
- `to` - Target entity ID

**Examples:**

```bash
rela unlink DEC-001 addresses REQ-001
```

---

### rela sync

Rebuild the graph from markdown files.

```bash
rela sync [flags]
```

**Flags:**

| Flag      | Description        |
| --------- | ------------------ |
| `--force` | Force full rebuild |

Use after manually editing markdown files to update the cache.

**Examples:**

```bash
rela sync
rela sync --force
```

---

### rela trace

Trace dependencies between entities.

#### rela trace from

Trace downstream dependencies (what depends on this entity).

```bash
rela trace from <id> [flags]
```

**Flags:**

| Flag      | Description                   |
| --------- | ----------------------------- |
| `--depth` | Maximum depth (0 = unlimited) |

**Examples:**

```bash
rela trace from REQ-001
rela trace from REQ-001 --depth 2
```

#### rela trace to

Trace upstream dependencies (what this entity depends on).

```bash
rela trace to <id> [flags]
```

**Flags:**

| Flag      | Description                   |
| --------- | ----------------------------- |
| `--depth` | Maximum depth (0 = unlimited) |

**Examples:**

```bash
rela trace to COMP-001
rela trace to COMP-001 --depth 3
```

#### rela trace path

Find the shortest path between two entities.

```bash
rela trace path <from> <to>
```

**Examples:**

```bash
rela trace path REQ-001 COMP-001
```

---

### rela graph

Export the entity graph to Graphviz DOT format.

```bash
rela graph [flags]
```

**Flags:**

| Flag           | Description                                             |
| -------------- | ------------------------------------------------------- |
| `-o, --output` | Output file (stdout if not specified)                   |
| `-f, --format` | Output format: `dot`, `png`, `svg`, `pdf`               |
| `--direction`  | Graph direction: `tb` (top-bottom) or `lr` (left-right) |
| `--types`      | Filter by entity types (comma-separated)                |

Rendering to PNG/SVG/PDF requires Graphviz (`dot` command).

**Examples:**

```bash
# Print DOT to stdout
rela graph

# Save DOT file
rela graph -o architecture.dot

# Render to PNG
rela graph -o architecture.png -f png

# Filter by types
rela graph --types requirement,decision

# Left-to-right layout
rela graph --direction lr
```

---

### rela export

Export entities to structured formats for external tool integration.

```bash
rela export [type] [flags]
```

**Arguments:**

- `type` - Entity type to export (required unless using `--all`)

**Flags:**

| Flag               | Description                                       |
| ------------------ | ------------------------------------------------- |
| `-f, --format`     | Output format: `json` (default), `csv`, or `yaml` |
| `--with-relations` | Include relation data in export                   |
| `--all`            | Export all entities and relations                 |

**Output Formats:**

- **JSON**: Array of objects with full property data. Best for programmatic processing with `jq`.
- **CSV**: Comma-separated values with header row. Best for spreadsheet import or `mlr` (Miller).
- **YAML**: YAML format. Best for human readability and configuration.

**Examples:**

```bash
# Export all controls as JSON
rela export control --format json

# Export controls as CSV for spreadsheet import
rela export control --format csv

# Export controls with their relations included
rela export control --with-relations

# Export all entities and relations
rela export --all --format json

# Use with jq for custom filtering
rela export control --format json | jq '.[] | select(.properties.status == "draft")'

# Use with jq to create a summary report
rela export control --format json | jq '[.[] | {id, title: .properties.title, status: .properties.status}]'

# Use with Miller for CSV filtering
rela export control --format csv | mlr --csv filter '$status == "implemented"'

# Generate a gap report (controls without evidence)
rela export control --with-relations --format json | \
  jq '.[] | select(.relations.outgoing.evidencedBy == null) | {id, title: .properties.title}'

# Export to file
rela export control --format csv > controls.csv
```

**Relation Data Structure:**

When using `--with-relations`, each entity includes:

```json
{
  "id": "CTRL-001",
  "type": "control",
  "properties": { ... },
  "relations": {
    "outgoing": {
      "mitigates": [{"id": "RISK-001", "title": "Unauthorized Access"}],
      "evidencedBy": [{"id": "EV-001", "title": "Audit Report"}]
    },
    "incoming": {
      "implements": [{"id": "PROC-001", "title": "Access Procedure"}]
    }
  }
}
```

**Full Export Structure:**

When using `--all`, the output includes both entities and relations:

```json
{
  "entities": [ ... ],
  "relations": [
    {"from": "DEC-001", "relation": "addresses", "to": "REQ-001"}
  ]
}
```

---

### rela fmt

Format entity and relation files for consistent styling.

```bash
rela fmt [type] [flags]
```

**Arguments:**

- `type` - Optional: specific entity type to format

**Flags:**

| Flag        | Description                                          |
| ----------- | ---------------------------------------------------- |
| `--dry-run` | Preview changes without writing                      |
| `--check`   | Check if files need formatting (exits 1 if they do)  |

This command normalizes:

- Frontmatter property ordering (id/type first for entities, from/relation/to for relations)
- Markdown content formatting (headings, lists, whitespace)
- Paragraph wrapping to configured line width (default: 80 characters)

**Configuration:**

Line width can be configured in `.rela/config.yaml`:

```yaml
formatting:
  line_width: 100  # default: 80
```

**What gets wrapped:**

- Paragraph text is wrapped to the configured line width
- Code blocks, headings, lists, and blockquotes are NOT wrapped (preserved as-is)

**Examples:**

```bash
rela fmt                # Format all entities and relations
rela fmt requirements   # Format only requirements (entities)
rela fmt --dry-run      # Preview changes without writing
rela fmt --check        # Check if files need formatting (for CI)
```

**CI Integration:**

Add to your CI pipeline to ensure consistent formatting:

```yaml
- run: rela fmt --check
```

This will exit with code 1 if any files need formatting.

---

### rela import

Import entities and relations from structured files.

```bash
rela import <file> [flags]
```

**Arguments:**

- `file` - Path to the import file (JSON, YAML, or CSV)

**Flags:**

| Flag              | Description                                                                           |
| ----------------- | ------------------------------------------------------------------------------------- |
| `-f, --format`    | Input format: `json`, `yaml`, or `csv`. Auto-detected from extension if not specified |
| `-n, --dry-run`   | Validate without creating files                                                       |
| `-u, --update`    | Replace existing entities instead of failing on duplicates                            |
| `--skip-errors`   | Continue importing on validation errors                                               |
| `-r, --relations` | Path to relations CSV file (for CSV imports)                                          |

**Input Formats:**

**JSON** - Object with `entities` and `relations` arrays, or just an array of entities:

```json
{
  "entities": [
    {
      "id": "REQ-001",
      "type": "requirement",
      "properties": { "title": "User login", "status": "draft" }
    },
    {
      "id": "DEC-001",
      "type": "decision",
      "properties": { "title": "Use JWT", "status": "accepted" }
    }
  ],
  "relations": [{ "from": "DEC-001", "relation": "addresses", "to": "REQ-001" }]
}
```

**YAML** - Same structure as JSON:

```yaml
entities:
  - id: REQ-001
    type: requirement
    properties:
      title: User login
      status: draft
relations:
  - from: DEC-001
    relation: addresses
    to: REQ-001
```

**CSV** - Columns for entity fields (id, type, and properties):

```csv
id,type,title,status,priority
REQ-001,requirement,User login,draft,high
REQ-002,requirement,User logout,draft,medium
```

For CSV, relations require a separate file with `--relations`:

```csv
from,relation,to
DEC-001,addresses,REQ-001
```

**Examples:**

```bash
# Import from JSON
rela import entities.json

# Import from YAML
rela import data.yaml

# Import from CSV
rela import entities.csv

# Import CSV with separate relations file
rela import entities.csv --relations relations.csv

# Dry-run to validate without creating files
rela import --dry-run data.json

# Update existing entities instead of failing
rela import --update data.json

# Continue on validation errors
rela import --skip-errors data.json

# Explicit format (when extension doesn't match)
rela import data.txt --format json
```

**Behavior Notes:**

- **Validation**: All entities are validated against the metamodel before import
- **Auto-generated properties**: If `status` is not provided, the entity type's default is used
- **Duplicate handling**: Without `--update`, importing an existing entity ID fails
- **Update mode**: `--update` does a full replacement, not a merge (existing properties not in the import file are removed)
- **Relations**: Relations referencing entities not in the graph (and not in the import) will fail
- **Atomic by default**: If any entity fails validation, no entities are created (unless `--skip-errors`)

**Round-trip with export:**

```bash
# Export all entities and relations
rela export --all -f json > backup.json

# Later, import to a new project
rela import backup.json
```

---

### rela analyze

Run quality analysis checks.

#### rela analyze orphans

Find entities with no connections.

```bash
rela analyze orphans
```

#### rela analyze duplicates

Find entities with similar titles.

```bash
rela analyze duplicates
```

#### rela analyze gaps

Find gaps in ID sequences for entity types with sequential IDs.

```bash
rela analyze gaps
```

Gap analysis only applies to entity types with `id_type: sequential`. Entity types with
`short` (default) or `manual` IDs are excluded since they don't follow numeric sequences.

#### rela analyze cardinality

Check relation cardinality constraints.

```bash
rela analyze cardinality
```

#### rela analyze validations

Run custom validation rules defined in the metamodel.

```bash
rela analyze validations
```

Validation rules check entity properties against custom conditions.
See [Metamodel Reference - Custom Validation Rules](metamodel.md#custom-validation-rules) for details.

**Example output:**

```text
$ rela analyze validations
✗ Accepted requirements must have a priority assigned (2):
  REQ-003: User authentication
  REQ-007: Data encryption
⚠ Decisions should have a rationale documented (1):
  DEC-002: Use PostgreSQL
Found 2 errors, 1 warnings across 2 rules
```

#### rela analyze properties

Validate entity property values against the metamodel schema.

```bash
rela analyze properties
```

Checks for:

- Invalid enum values (not in allowed list)
- Invalid custom type values
- Invalid date formats
- Invalid integer/boolean values
- Missing required properties
- Entity IDs not matching configured patterns

This catches issues in manually-edited markdown files that bypass CLI validation.

#### rela analyze schema

Analyze metamodel schema usage to find unused or underused types.

```bash
rela analyze schema
rela analyze schema --threshold 2
rela analyze schema --cleanup --dry-run
```

Shows:

- Entity types with no instances
- Relation types with no instances
- Custom types (enums) not referenced by any property
- Types with few instances (when `--threshold` is set)

**Flags:**

| Flag          | Description                                                    |
| ------------- | -------------------------------------------------------------- |
| `--threshold` | Show types with instance count <= threshold (0 = only unused)  |
| `--cleanup`   | Remove unused types from metamodel.yaml                        |
| `--dry-run`   | Preview cleanup changes without modifying files                |

The cleanup operation only removes types that have no instances AND no references in
configuration files (data-entry.yaml, validations, automations). Types referenced in
forms, lists, views, or validations will not be removed even if they have zero instances.

#### rela analyze all

Run all analysis checks.

```bash
rela analyze all
```

Runs orphans, duplicates, gaps, cardinality, properties, and (if defined) custom validations.

---

### rela mcp

Start the MCP (Model Context Protocol) server over stdio.

```bash
rela mcp
```

Exposes rela's capabilities to AI assistants like Claude Code, Cursor, and other MCP-compatible
clients. The server runs on stdin/stdout using JSON-RPC and provides:

- **21 tools** for entity/relation CRUD, graph tracing, analysis, and export
- **3 resources** for reading entities, relations, and the metamodel by URI
- **4 prompts** for common AI-assisted workflows
- **File watching** with automatic graph sync on changes

**Client Configuration (Claude Code):**

Setup with `claude mcp add` (recommended):

```bash
claude mcp add rela -s local -- /path/to/rela mcp
```

Setup with `.mcp.json` (for sharing via git):

```json
{
  "mcpServers": {
    "rela": {
      "command": "rela",
      "args": ["mcp"]
    }
  }
}
```

See [MCP Server Guide](mcp-server.md) for the full tool/resource/prompt reference.

---

### rela tui

Launch the interactive terminal UI.

```bash
rela tui
```

See [TUI Guide](tui.md) for details.

---

### rela completion

Generate shell completion scripts.

```bash
rela completion <shell>
```

**Arguments:**

- `shell` - Target shell: `bash`, `zsh`, `fish`, or `powershell`

**Examples:**

```bash
# Bash
rela completion bash > /etc/bash_completion.d/rela

# Zsh
rela completion zsh > "${fpath[1]}/_rela"

# Fish
rela completion fish > ~/.config/fish/completions/rela.fish
```

---

### rela template

Manage templates for creating entities and relations.

Templates provide default frontmatter values and markdown body content when creating new entities or
relations. They are stored in:

- `templates/entities/<type>.md` - Entity templates
- `templates/relations/<type>.md` - Relation templates

#### rela template init

Generate template files from the metamodel.

```bash
rela template init [type...] [flags]
```

**Arguments:**

- `type...` - Optional: specific entity or relation types to generate templates for

**Flags:**

| Flag          | Description                      |
| ------------- | -------------------------------- |
| `--entities`  | Only generate entity templates   |
| `--relations` | Only generate relation templates |
| `--force`     | Overwrite existing templates     |

Without arguments, generates templates for all entity and relation types defined in the metamodel.

**Examples:**

```bash
# Generate all templates
rela template init

# Generate template for a specific entity type
rela template init requirement

# Generate template for a specific relation type
rela template init addresses

# Generate only entity templates
rela template init --entities

# Generate only relation templates
rela template init --relations

# Overwrite existing templates
rela template init --force

# Generate specific types with force
rela template init requirement decision --force
```

**Generated Template Format:**

Entity templates include all properties from the metamodel with their default values:

```markdown
---
title: ""
status: draft
priority: medium
---

# Description

Describe your requirement here.
```

Relation templates include a placeholder for rationale:

```markdown
---
---

# Rationale

Explain why this addresses relation exists.
```

**Using Templates:**

Once templates are created, they are automatically applied when using `rela create` or `rela link`.
CLI flags override template defaults.

```bash
# Create a template
rela template init requirement

# Edit the template to customize defaults
# templates/entities/requirement.md

# New entities will use the template
rela create requirement --title "My Requirement"
```

---

### rela migrate

Migrate project files to the current schema format.

```bash
rela migrate [flags]
```

**Flags:**

| Flag      | Description                                                   |
| --------- | ------------------------------------------------------------- |
| `--check` | Check for pending migrations without applying (useful for CI) |

This command detects deprecated syntax patterns in your project files (e.g., `metamodel.yaml`) and
transforms them to the current format while preserving comments and formatting.

**When to use:**

If you see an error like this when running any rela command:

```text
metamodel.yaml uses deprecated syntax:
  - Rename id_type values: "sequential" → "auto", "string" → "manual"

Run 'rela migrate' to update your project files.
```

Run `rela migrate` to automatically update your files.

**Examples:**

```bash
# Apply all pending migrations
rela migrate

# Check for migrations without applying (for CI pipelines)
rela migrate --check
```

**CI Integration:**

Add to your CI pipeline to ensure project files are up-to-date:

```yaml
- run: rela migrate --check
```

This will exit with code 1 if migrations are needed.

---

### rela normalize

Normalize markdown headers in entity files to start at level 2 (##).

```bash
rela normalize [type] [flags]
```

This command adjusts header levels so the minimum header level in each entity is `##`,
preserving the relative hierarchy. For example:

```markdown
# Overview        →  ## Overview
## Details        →  ### Details
### Subsection    →  #### Subsection
```

Setext-style headers (underlined with `===` or `---`) are converted to ATX style (`##`).

**Arguments:**

- `type` - Optional: specific entity type to normalize

**Flags:**

| Flag        | Description                        |
| ----------- | ---------------------------------- |
| `--dry-run` | Preview changes without writing    |

**Examples:**

```bash
rela normalize                # Normalize all entities
rela normalize requirements   # Normalize only requirements
rela normalize req            # Alias works too
rela normalize --dry-run      # Preview changes without writing
```

---

### rela rename

Rename entity types or entity IDs across the project.

#### rela rename entity

Rename an entity type across the entire project.

```bash
rela rename entity <old-type> <new-type> [flags]
```

**Arguments:**

- `old-type` - Current entity type name
- `new-type` - New entity type name

**Flags:**

| Flag           | Description                              |
| -------------- | ---------------------------------------- |
| `--plural`     | Override plural form for directory name  |
| `-f, --force`  | Skip confirmation prompt                 |

This updates:

- The entity key in `metamodel.yaml`
- All relation `from`/`to` references in `metamodel.yaml`
- All validation `entity_type` references in `metamodel.yaml`
- The entity directory (e.g., `entities/issues/` → `entities/tickets/`)
- The `type` field in all entity markdown files
- Entity templates (if they exist)

**Examples:**

```bash
rela rename entity issue ticket
rela rename entity issue ticket --plural tickets
rela rename entity requirement feature --force
```

#### rela rename id

Rename an entity's ID and update all relations that reference it.

```bash
rela rename id <old-id> <new-id> [flags]
```

**Arguments:**

- `old-id` - Current entity ID
- `new-id` - New entity ID

**Flags:**

| Flag        | Description                      |
| ----------- | -------------------------------- |
| `--dry-run` | Preview changes without applying |

This updates:

- The entity file (renamed and `id` field updated)
- All relation files where this entity is the `from` or `to` endpoint

**Examples:**

```bash
rela rename id REQ-001 REQ-100
rela rename id REQ-001 REQ-100 --dry-run
```

---

### rela schema

View the metamodel schema.

```bash
rela schema [command] [flags]
```

Displays information about the loaded metamodel including entity types, relation types,
and custom types.

**Subcommands:**

| Command     | Description                              |
| ----------- | ---------------------------------------- |
| `overview`  | Show metamodel overview (default)        |
| `entities`  | List all entity types with descriptions  |
| `relations` | List all relation types                  |
| `types`     | List custom types defined in metamodel   |
| `entity`    | Show details for a specific entity type  |
| `relation`  | Show details for a specific relation     |

**Flags:**

| Flag            | Description                                     |
| --------------- | ----------------------------------------------- |
| `--graphviz`    | Output metamodel as GraphViz DOT format         |
| `--constraints` | Include cardinality constraints in DOT output   |

**Examples:**

```bash
rela schema                    # Overview
rela schema entities           # List entity types
rela schema relations          # List relation types
rela schema types              # List custom types
rela schema entity service     # Detail for one entity type
rela schema relation addresses # Detail for one relation type
rela schema --graphviz         # Output as DOT format
rela schema --graphviz --constraints  # DOT with cardinality
```

---

### rela gc

Garbage collect unreferenced files from the project.

```bash
rela gc [flags]
```

**Flags:**

| Flag            | Description                                              |
| --------------- | -------------------------------------------------------- |
| `--attachments` | Clean up unreferenced attachment files                   |
| `--temp-files`  | Clean up orphaned `.new` files from interrupted writes   |
| `--dry-run`     | Show what would be removed without actually removing     |

**Examples:**

```bash
rela gc --attachments           # Remove unreferenced attachment files
rela gc --temp-files            # Remove orphaned temp files
rela gc --attachments --dry-run # Preview what would be removed
```

---

### rela validate

Validate project configuration files.

```bash
rela validate
```

Checks `metamodel.yaml` and `data-entry.yaml` for:

- Unknown/misspelled keys
- Invalid cross-references (forms, lists, views)
- Invalid entity types, relations, and properties
- View traversal correctness
- Dashboard and command configuration

**Examples:**

```bash
rela validate
```

---

### rela version

Print version information.

```bash
rela version
```
