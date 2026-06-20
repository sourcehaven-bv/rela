<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Data Entry Web App

The data entry application provides a web-based UI for creating, editing, and browsing entities
stored in a rela project. It is configured entirely through a `data-entry.yaml` file placed
alongside your `metamodel.yaml`.

## Overview

A `data-entry.yaml` file defines:

- **App metadata** - Name and description shown in the UI
- **Git settings** - Protected branches that require pull requests
- **Styles** - Color mappings for enum values displayed in lists and forms
- **Forms** - Create and edit forms for entity types, with fields and relation pickers
- **Lists** - Tabular views with sorting, filtering, and pagination
- **Views** - Read-only detail pages that traverse the graph to show related entities
- **Dashboard** - An overview page with query-driven cards showing counts, breakdowns, and tables
- **Kanbans** - Visual board views with drag-and-drop cards grouped by columns and optional swimlanes
- **Navigation** - Sidebar menu entries with optional grouping
- **Actions** - Quick operations (property mutations or Lua scripts) triggered from lists or the sidebar
- **Commands** - User-defined scripts triggered from the UI with streamed results
- **Documents** - Read-only rendered markdown panels attached to entity views, composed via shell commands or Lua scripts
- **User Defaults** - Per-user default values for properties and relations, configurable via Settings page

The file drives the entire UI without writing any code. The server reads `data-entry.yaml` and
your `metamodel.yaml` together, validates them, and serves a fully functional CRUD application.

## Quick Start

### 1. Create data-entry.yaml

Place a `data-entry.yaml` in your project root (next to `metamodel.yaml`):

```yaml
version: "1.0"

app:
  name: "My Project"
  description: "Project management system"

forms:
  create_task:
    entity_type: task
    title: "New Task"
    body: true
    fields:
      - property: title
        label: "Title"
        required: true
      - property: status
        label: "Status"
        default: open

lists:
  all_tasks:
    entity_type: task
    title: "All Tasks"
    columns:
      - property: title
        label: "Title"
        sortable: true
        link: true
      - property: status
        label: "Status"
        sortable: true
    create_form: create_task
    page_size: 25

navigation:
  - label: "Tasks"
    list: all_tasks
```

### 2. Start the Server

```bash
rela-server -project /path/to/project
```

Or with a custom config path:

```bash
rela-server -project /path/to/project -config /path/to/data-entry.yaml
```

The server starts on port 8080 by default. Open `http://localhost:8080` in your browser.

## File Structure

```yaml
version: "1.0"            # Config format version

app:                       # Application metadata
  name: "..."
  description: "..."

git:                       # Git sync settings
  require_pr: [main]

styles:                    # Color mappings for enum values
  status:
    open: blue
    closed: gray

forms:                     # Create/edit form definitions
  form_name:
    entity_type: task
    ...

lists:                     # List view definitions
  list_name:
    entity_type: task
    ...

views:                     # Detail view definitions
  view_name:
    entry:
      type: task
    ...

dashboard:                 # Optional overview page
  title: "Dashboard"
  cards:
    - title: "Open"
      query: "type:task status:open"
      display: count

kanbans:                   # Kanban board views
  board_name:
    entity_type: task
    column_property: status
    ...

commands:                  # User-defined scripts
  export-json:
    label: "Export JSON"
    script: "jq '.' > /tmp/export.json"
    context: entity

navigation:                # Sidebar menu (supports groups)
  - label: "Dashboard"
    dashboard: true
  - group: "Tasks"
    items:
      - label: "All Tasks"
        list: all_tasks
```

## App

Display metadata shown in the header:

```yaml
app:
  name: "Support Tickets"
  description: "Internal ticket management system"
```

| Field         | Description                      |
| ------------- | -------------------------------- |
| `name`        | Application title in the header  |
| `description` | Subtitle shown below the title   |

## Git

Configure git synchronization behavior:

```yaml
git:
  enabled: true
  mode: direct              # "direct" or "pr"
  branch: main              # Branch to sync with (direct mode)
  base_branch: main         # Branch to rebase onto (pr mode)
  push_branch: feature/data # Branch to push to (pr mode)
  fetch_interval: 30        # Background fetch interval in seconds (0 = disabled)
  require_pr: [main, production]
```

| Field            | Description                                                           |
| ---------------- | --------------------------------------------------------------------- |
| `enabled`        | Enable git sync features (status bar, sync button)                    |
| `mode`           | `direct` pushes to the same branch; `pr` rebases onto base and pushes to a separate branch |
| `branch`         | Target branch for direct mode (default: `main`)                       |
| `base_branch`    | Branch to rebase onto in PR mode                                      |
| `push_branch`    | Branch to push to in PR mode                                          |
| `fetch_interval` | Seconds between background fetches (0 disables background fetch)      |
| `require_pr`     | List of branch names where direct push is blocked (protected branches) |

### Sync behavior

When git is enabled, the UI shows a status bar with:

- Current branch name
- Number of local changes (uncommitted files in `entities/` and `relations/`)
- Number of remote commits ahead
- Conflict indicator if a rebase conflict is in progress

The **Sync** button performs:

1. Stage all changes in `entities/` and `relations/`
2. Commit with an auto-generated message describing the changes
3. Fetch from remote
4. Rebase onto the target branch (if behind)
5. Push to the remote

If a rebase conflict occurs, the status bar shows a conflict indicator and provides options to
resolve conflicts or abort the rebase.

When editing on a protected branch, the UI shows a banner suggesting the user create a working
branch. Commits are auto-created on every entity change, but push is blocked until the user
switches to a non-protected branch.

## Styles

Map enum values to colors for visual display in lists and forms:

```yaml
styles:
  status:
    draft: gray
    review: blue
    approved: green
    active: green
    retired: gray

  priority:
    critical: red
    high: orange
    medium: yellow
    low: green
```

The key is the custom type name (as defined in `metamodel.yaml` under `types:`). Each value maps
to a color name. These colors are applied everywhere that enum value appears: list cells, badges,
and form select options.

**Available colors:** `red`, `orange`, `yellow`, `green`, `blue`, `purple`, `gray`.

## Display names

Every entity's display name — the human-readable string shown in
lists, cards, side-panel breadcrumbs, related-entity links, and
search results — comes from the entity type's *primary property*.
Set it with `display_property` in `metamodel.yaml`:

```yaml
# metamodel.yaml
entities:
  applicatie:
    label: Applicatie
    display_property: naam
    properties:
      naam:
        type: string
        required: true
```

Without `display_property`, rela auto-derives one from
`title` / `name` / `label` (then any required string property,
alphabetical). That's brittle for non-English schemas — pin it
explicitly. See [GUIDE-metamodel.md → Display
name](metamodel.md#display-name) for the metamodel-side rules
(allowed types, validation diagnostics).

Where the display name shows up in the data-entry app:

- **List columns**: a column with `link: detail` renders the entity's
  display name as the link text.
- **Cards**: card titles (in `display: cards` sections, kanban cards,
  related-entity widgets).
- **Breadcrumbs**: the side panel and form headers show the display
  name above the ID.
- **Related-entity links**: every relation widget that renders linked
  entities uses the display name as link text.
- **Search results**: each result row shows the display name first.

When the display value is empty, missing, or `nil`, the UI falls
back to the entity ID — never an empty string.

## Forms

Forms define the UI for creating and editing entities. Each form is a named entry under `forms:`.

### Basic Form

```yaml
forms:
  create_ticket:
    entity_type: ticket
    title: "New Ticket"
    description: "Submit a new support ticket"
    body: true

    fields:
      - property: title
        label: "Title"
        placeholder: "Brief summary..."
        required: true

      - property: priority
        label: "Priority"
        default: medium

    relations:
      - relation: belongs-to
        direction: outgoing
        target_type: category
        label: "Category"
        widget: select
```

### Form Fields

| Field            | Type   | Description                                               |
| ---------------- | ------ | --------------------------------------------------------- |
| `entity_type`    | string | Entity type this form operates on (must exist in metamodel) |
| `title`          | string | Form heading                                              |
| `description`    | string | Help text shown below the heading                         |
| `mode`           | string | `"edit"` for edit forms (omit for create forms)           |
| `body`           | bool   | Show a markdown body editor                               |
| `fields`         | list   | Property fields                                           |
| `relations`      | list   | Relation picker fields                                    |

### Field Options

Each entry in `fields:` configures one property input:

| Field         | Type              | Description                                                    |
| ------------- | ----------------- | -------------------------------------------------------------- |
| `property`    | string            | Property name from the metamodel                               |
| `label`       | string            | Display label (defaults to property name)                      |
| `placeholder` | string            | Placeholder text for empty inputs                              |
| `help`        | string            | Help text shown below the field                                |
| `required`    | bool              | Field must be filled before submission                         |
| `default`     | string            | Default value for new entities                                 |
| `hidden`      | bool              | Include in form data but hide from UI                          |
| `widget`      | string            | Input widget type (see below)                                  |
| `transitions` | map[string]list   | Allowed state transitions for enum fields (edit forms only)    |

### Widget Types

| Widget     | Description                                      | Use For                        |
| ---------- | ------------------------------------------------ | ------------------------------ |
| *(default)* | Auto-detected from property type                | Strings, enums                 |
| `text`     | Single-line text input                           | Short strings                  |
| `textarea` | Multi-line text area                             | Descriptions, notes            |
| `number`   | Numeric input                                    | Integers                       |
| `date`     | Date picker                                      | Date properties                |
| `checkbox` | Toggle checkbox                                  | Boolean properties             |

When no widget is specified, the system auto-detects from the property's type in the metamodel:
enum types render as a `<select>`, booleans as checkboxes, dates as date pickers, and everything
else as text inputs.

### ID Controls on Create Forms

Create forms adapt to the entity type's `id_type` and `id_prefix` / `id_prefixes`
configuration in the metamodel. The user-facing UI is generated automatically —
no extra form configuration is required.

- **Single-prefix types** (`id_prefix: "TKT-"`): no extra controls. The form
  submits and the server assigns the next ID.
- **Multi-prefix types** (`id_prefixes: ["DEC-", "ADR-"]`): the create form
  renders a **Prefix** dropdown so the user picks which prefix the new entity
  should use. The server validates the chosen prefix against the declared
  list — unknown values are rejected with a 422.
- **Manual ID types** (`id_type: manual`): the create form renders a required
  **ID** text input that is sent verbatim as the entity's ID. If the type also
  declares one or more prefixes, the supplied ID must start with one of them
  and include a non-empty suffix. The edit form shows the ID as a read-only
  display; renaming uses the dedicated rename flow.

### State Transitions

For edit forms, you can restrict which enum values are selectable based on the current value:

```yaml
fields:
  - property: status
    label: "Status"
    transitions:
      open: [in-progress, closed]
      in-progress: [open, resolved]
      resolved: [closed, in-progress]
      closed: [open]
```

Each key is a current value; its list contains the values the user can transition to. The current
value is always implicitly included. If `transitions` is omitted, all enum values are shown.

### Relation Fields

Each entry in `relations:` configures a relation picker:

| Field          | Type   | Description                                                    |
| -------------- | ------ | -------------------------------------------------------------- |
| `relation`     | string | Relation type name from the metamodel                          |
| `direction`    | string | `"outgoing"` or `"incoming"`                                   |
| `target_type`  | string | Entity type of the related entity                              |
| `label`        | string | Display label                                                  |
| `required`     | bool   | At least one relation must be selected                         |
| `widget`       | string | `"select"`, `"multi-select"`, `"cards"`, or `"search"` (auto-detected) |
| `allow_create` | bool   | Show an inline "create new" button                             |
| `create_form`  | string | Form name to use for inline creation                           |
| `properties`   | list   | Editable properties on the relation (only with `cards` widget) |

**Relation widget types:**

| Widget         | Description                                                  |
| -------------- | ------------------------------------------------------------ |
| `select`       | Dropdown listing all entities of the target type (pick one)  |
| `multi-select` | Tag-style picker for selecting multiple entities             |
| `cards`        | Card-based UI with inline property editing (auto-selected when relation has properties or content) |
| `search`       | Type-ahead search field for large entity sets                |

Widget is auto-detected based on metamodel: if the relation type has `properties` or `content: true` defined,
the UI uses `cards`. Otherwise, cardinality determines `select` vs `multi-select`.

**Inline creation:** When `allow_create: true` and `create_form` is set, a button appears next to
the relation picker. Clicking it opens a modal with the referenced form, and the newly created
entity is automatically linked.

### Reverse (incoming) Relations

Relation types are directional in the metamodel: `implements` goes from `task` to `feature`.
Often you want to show the *inbound* side on the opposite entity's form — on the feature form,
"which tasks implement me?". Use `direction: incoming` to render a reverse widget:

```yaml
forms:
  feature:
    entity_type: feature
    relations:
      # Show tasks that implement this feature (incoming 'implements' edges).
      - relation: implements
        direction: incoming
        label: "Implemented by"
```

When `direction: incoming` is set:

- The widget reads edges via `GET /api/v1/{plural}/{id}/relations/{relType}?direction=incoming`.
- The target-type list comes from the relation's `from:`, not `to:`.
- Cardinality (single vs. multi) honors the relation's `max_incoming` instead of `max_outgoing`.
- Saving a new link writes the edge as `(peer) → {relType} → (current entity)`; the backend
  swaps from/to so the on-disk relation file stays canonical.
- Grouped responses from `GET /api/v1/{plural}/{id}/relations` surface incoming edges under
  the relation's `inverse:` name (see [metamodel.md](metamodel.md#inverse-relations)), e.g.
  `blocks` → `blockedBy`.

All form widgets (`select`, `multi-select`, `search`, `cards`) honor `direction: incoming`.

**Label collision:** The widget's section heading defaults to `label || relation`. If you
put two widgets with the same relation and no `label:` next to each other (one outgoing, one
incoming), they'll both render as "blocks". Always set an explicit `label:` on reverse
widgets — e.g. `"Blocked by"`.

### Relation Properties

When a relation type has `properties` defined in the metamodel, the `cards` widget is automatically
used and you can configure which properties are editable in the form:

```yaml
relations:
  - relation: blocks
    direction: outgoing
    target_type: ticket
    label: "Blocks"
    # widget: cards  (auto-selected because 'blocks' has properties in metamodel)
    properties:
      - property: reason
        label: "Block Reason"
        widget: text
```

| Field      | Type   | Description                       |
| ---------- | ------ | --------------------------------- |
| `property` | string | Relation property name            |
| `label`    | string | Display label                     |
| `widget`   | string | Input widget (`text`, `textarea`) |
| `required` | bool   | Must be filled                    |

### Help Modal

Every form displays a help icon (?) next to the title. Clicking it opens a modal with
documentation for the entity type, pulled from the metamodel:

- **Entity description**: The `description` field from the entity definition (supports markdown)
- **Properties**: All properties with their types and descriptions
- **Outgoing relations**: Relations from this entity to others, with cardinality constraints
- **Incoming relations**: Relations from other entities to this one, with cardinality constraints

Relations with minimum cardinality >= 1 are marked as "required" in the help modal, indicating
that at least one relation of that type must be created.

To populate the help modal, add descriptions to your metamodel:

```yaml
entities:
  ticket:
    label: Ticket
    description: |
      A ticket represents a unit of work to be completed.

      Use tickets for:
      - Bug reports
      - Feature requests
      - Tasks and chores
    properties:
      title:
        type: string
        required: true
        description: "Brief summary of the ticket"
      priority:
        type: priority
        description: "How urgently this ticket needs attention"

relations:
  blocks:
    label: blocks
    description: "Indicates this ticket must be resolved before another can proceed"
    from: [ticket]
    to: [ticket]
    min_outgoing: 0
    max_outgoing: 10
```

## Lists

Lists display entities in a sortable, filterable table with optional create/edit actions.

### Basic List

```yaml
lists:
  all_tickets:
    entity_type: ticket
    title: "All Tickets"
    description: "View all tickets"

    columns:
      - property: title
        label: "Title"
        sortable: true
        link: true
      - property: status
        label: "Status"
        sortable: true
      - property: priority
        label: "Priority"
        sortable: true

    sort:
      property: priority
      direction: asc

    create_form: create_ticket
    edit_form: edit_ticket
    page_size: 25
```

> **Where does a click on a row go?** That's configured at entity-type
> granularity in the top-level `entity_views:` block — see
> [Entity Views](#entity-views) below. Per-list `detail_view` is no longer
> used; if you have one in an existing config, run `rela migrate` and it
> will be moved automatically.

### List Fields

| Field             | Type   | Description                                                 |
| ----------------- | ------ | ----------------------------------------------------------- |
| `entity_type`     | string | Entity type to list                                         |
| `title`           | string | List heading                                                |
| `description`     | string | Subtitle                                                    |
| `columns`         | list   | Column definitions                                          |
| `sort`            | object | Default sort order                                          |
| `filters`         | list   | Static filters (always applied)                             |
| `filter_controls` | list   | Interactive filter controls shown to the user               |
| `create_form`     | string | Form name for the "New" button                              |
| `edit_form`       | string | Form name for the row edit action                           |
| `page_size`       | int    | Rows per page (default: 25)                                 |
| `actions`         | list   | Action IDs available as keyboard shortcuts on selected rows |

### Column Options

A column shows either a property value or the comma-separated titles of an entity's related
entities — set exactly one of `property` or `relation`.

| Field       | Type   | Description                                                                 |
| ----------- | ------ | --------------------------------------------------------------------------- |
| `property`  | string | Property name to display                                                    |
| `relation`  | string | Relation type whose targets are shown comma-separated                       |
| `direction` | string | Relation columns only: `"outgoing"` (default) or `"incoming"` for reverse   |
| `label`     | string | Column header (defaults to property / relation name)                        |
| `sortable`  | bool   | Column can be sorted by clicking the header                                 |
| `link`      | bool   | Cell value links to the entity's detail page                                |

**Reverse relation column example** — on a feature list, show which tasks implement each row:

```yaml
columns:
  - property: title
    link: true
  - relation: implements
    direction: incoming
    label: "Implemented by"
```

### Static Filters

Apply filters that are always active (the user cannot remove them):

```yaml
filters:
  - property: status
    operator: "="
    value: open
```

| Field      | Type   | Description                              |
| ---------- | ------ | ---------------------------------------- |
| `property` | string | Property to filter on                    |
| `operator` | string | See operators below                      |
| `value`    | string | Value to compare against                 |

**Operators:**

| Operator   | Type support              | Behavior                                              |
| ---------- | ------------------------- | ----------------------------------------------------- |
| `=`        | string, enum              | Exact match                                           |
| `!=`       | string, enum              | Not equal; supports comma-separated values (NOT IN)   |
| `~`        | string                    | Substring match (case-insensitive)                    |
| `<`, `<=`  | date, number              | Less than / less than or equal                        |
| `>`, `>=`  | date, number              | Greater than / greater than or equal                  |
| `in`       | string, enum              | Comma-separated list; matches any                     |

The ordering operators (`<`, `<=`, `>`, `>=`) compare with type-aware
parsing: dates are tried first (`YYYY-MM-DD`), then numbers, then string
comparison. If one side parses as a date or number and the other doesn't,
the comparison is **rejected** (the entity is excluded) — there is no
silent lexicographic fallback.

**Variable substitution in filter values:**

Filter values starting with `$` are reserved for variables. The following
date variables are supported:

| Variable     | Resolves to                          |
| ------------ | ------------------------------------ |
| `$today`     | Today's date in `YYYY-MM-DD` (UTC)   |
| `$tomorrow`  | Tomorrow's date                      |
| `$yesterday` | Yesterday's date                     |

Variables are evaluated in **UTC** for predictability across server
timezones. Variables work in single-value operators and in comma-separated
lists (`in`, `!=`):

```yaml
filters:
  # Show overdue tasks
  - property: due_date
    operator: "<="
    value: $today

  # Multiple variable tokens in a list
  - property: due_date
    operator: in
    value: "$yesterday,$today,$tomorrow"
```

To filter for a literal value that starts with `$`, you currently cannot
escape it — choose property values that don't start with `$`.

### Filter Controls

Interactive filters shown above the table:

```yaml
filter_controls:
  - property: status
    widget: multi-select
  - property: priority
    widget: select
  - property: assignee
    widget: search
```

| Field      | Type   | Description                                              |
| ---------- | ------ | -------------------------------------------------------- |
| `property` | string | Property to filter on                                    |
| `widget`   | string | `"select"`, `"multi-select"`, or `"search"`             |

### URL Sync for Filters

Interactive filter selections are mirrored into the page's URL query string so
lists are deep-linkable and survive browser back/forward. The format is
bracketed:

```text
/list/all_tasks?filter[status]=open
/list/all_tasks?filter[due_date][lte]=$today
/list/all_tasks?filter[tags][in][]=urgent&filter[tags][in][]=blocker
```

Rules:

- The implicit equality form (`filter[prop]=value`) is the most concise; it
  matches the API's default `eq` operator.
- Operator suffixes (`[lte]`, `[gt]`, `[contains]`, `[in]`, …) follow the same
  names as the REST API operators. The full list is `eq`, `ne`, `contains`,
  `in`, `lt`, `lte`, `gt`, `gte` — see the ["Static Filters"](#static-filters)
  section above and the `applyV1Filters` source in
  `internal/dataentry/api_v1.go` for semantics.
- Unknown operators (typos like `[equals]`) are **skipped** with a server-side
  warning rather than treated as a pass-all fallback. This is a deliberate
  fail-closed behavior so a typo can't silently bypass a configured scope.
- Multi-value filters use the repeated array form (`filter[prop][in][]=a&…`).
  Only `in` and `ne` join all repeated values; other operators take
  last-write-wins if a key appears multiple times.
- Static `filters:` entries (the always-active list config above) take
  precedence: a URL filter on the same property is dropped with a console
  warning rather than silently overriding the locked scope. **Important:**
  the lock is whole-property granularity, not per-operator — a static
  `filter[date][gte]=2024-01-01` blocks *any* URL filter on `date`,
  including `filter[date][lte]`. If you need a range combined with a static
  lower bound, define both bounds in `data-entry.yaml` rather than via URL.
- Text-input filters debounce at 250ms — typing into a search filter only
  fires one backend request after you stop typing, not one per keystroke.
- Clearing all filters from the FilterBar removes every `filter[*]` param
  from the URL while preserving non-filter params (`from`, `sort`, `page`,
  `scope`).

### Sort Configuration

Sort supports multiple criteria as a list. The first entry is the primary sort key:

```yaml
sort:
  - property: priority
    direction: desc
  - property: due_date
    direction: asc   # "asc" (default) or "desc"
```

You can also sort by the virtual properties `id` (entity ID) and `modified` (file modification time).

If no sort is configured, the list falls back to the entity type's `default_sort` from the metamodel,
or sorts by ID ascending.

The search bar also supports `sort:` clauses (see [Query Syntax](#query-syntax) below).

> **Migration**: If your config uses the old single-object format (`sort: {property: ..., direction: ...}`),
> run `rela migrate` to convert it to the list format.

## Views

Views define read-only detail pages that traverse the entity graph to display related data,
adapted for rendering as HTML sections.

### Basic View

```yaml
views:
  ticket_report:
    title: "Ticket Report"
    entry:
      type: ticket

    traverse:
      - from: entry
        follow: blocks
        collect_as: blocked_tickets
      - from: entry
        follow_incoming: blocks
        collect_as: blocked_by
      - from: entry
        follow: tagged
        collect_as: labels

    sections:
      - heading: "Ticket"
        source: entry
        display: properties
        fields:
          - property: status
          - property: priority
          - property: assignee

      - source: entry
        display: content

      - heading: "Blocks"
        source: blocked_tickets
        display: table
        columns:
          - property: title
            label: "Title"
            link: true
          - property: status
            label: "Status"
        empty_message: "No blocked tickets"
```

### View Fields

| Field      | Type   | Description                                    |
| ---------- | ------ | ---------------------------------------------- |
| `title`    | string | Page heading                                   |
| `entry`    | object | Entry entity type                              |
| `traverse` | list   | Graph traversal rules                          |
| `sections` | list   | Display sections                               |

### Entry

```yaml
entry:
  type: ticket   # Entity type of the entry entity
```

When a user opens a view, the entry entity is determined by the URL. For example,
clicking a list row whose `entity_type` has `entity_views.ticket.detail_view: ticket_report`
opens the view for that specific ticket entity.

### Traverse Rules

Traverse rules collect related entities into named collections:

```yaml
traverse:
  # Follow outgoing relations
  - from: entry
    follow: blocks
    collect_as: blocked_tickets

  # Follow incoming relations
  - from: entry
    follow_incoming: tagged
    collect_as: labels

  # Chain from a previous collection
  - from: blocked_tickets
    follow: tagged
    collect_as: blocked_labels

  # Recursive traversal
  - from: entry
    follow: dependsOn
    recursive: true
    max_depth: 5
    collect_as: dependencies

  # Filter results with where clause
  - from: entry
    follow_incoming: partOf
    collect_as: functions
    where: "type = function"

  # Filter by property value
  - from: entry
    follow_incoming: partOf
    collect_as: active_items
    where: "status = active"
```

| Field             | Type   | Description                                        |
| ----------------- | ------ | -------------------------------------------------- |
| `from`            | string | Source: `"entry"` or a collection name              |
| `follow`          | string | Outgoing relation type to follow                   |
| `follow_incoming` | string | Incoming relation type to follow (reverse)         |
| `collect_as`      | string | Name for the collected entities                    |
| `recursive`       | bool   | Follow the relation transitively                   |
| `max_depth`       | int    | Maximum recursion depth                            |
| `where`           | string | Filter expression to select matching entities      |

#### Where Clause Syntax

The `where` clause filters traversed entities using simple expressions:

```text
property = value    # Equality
property != value   # Inequality
```

The special pseudo-property `type` matches the entity type:

```yaml
where: "type = function"     # Only function entities
where: "type != component"   # Everything except components
```

Regular properties use metamodel-aware matching:

```yaml
where: "status = active"     # Match status property
where: "priority != low"     # Exclude low priority
```

If a where clause is invalid or a property doesn't exist, the filter is silently
skipped and all entities are returned (fail-open for robustness).

### Sections

Sections define how collected entities are rendered on the page:

```yaml
sections:
  - heading: "Properties"
    source: entry
    display: properties
    fields:
      - property: status
      - property: priority
        label: "Priority Level"

  - heading: "Description"
    source: entry
    display: content

  - heading: "Related Items"
    source: related_items
    display: table
    columns:
      - property: title
        label: "Title"
        link: true
      - property: status
        label: "Status"
    empty_message: "No related items found"
```

| Field           | Type   | Description                                             |
| --------------- | ------ | ------------------------------------------------------- |
| `heading`       | string | Section heading (optional; omit for no heading)         |
| `source`        | string | `"entry"` or a traverse collection name                 |
| `display`       | string | Display mode (see below)                                |
| `fields`        | list   | Properties to show (`properties`, `content`, `cards`, `list` modes) |
| `columns`       | list   | Column definitions (`table` mode)                       |
| `group_by`      | string | Property to group entities by                           |
| `empty_message` | string | Text shown when the collection is empty                 |
| `link`          | bool   | Link entity titles to their detail pages                |

### Display Modes

| Mode         | Description                                                     |
| ------------ | --------------------------------------------------------------- |
| `properties` | Key-value pairs rendered as a definition list                   |
| `content`    | Markdown body of the entity rendered as HTML                    |
| `table`      | Tabular layout with configurable columns (like a mini-list)     |
| `cards`      | Card layout showing each entity with selected property badges   |
| `list`       | Simple bulleted list of entity titles with optional fields      |

**`properties`** is best for the entry entity's metadata. **`content`** renders the markdown body.
**`table`** works well for collections with many items. **`cards`** provides a visual layout for
smaller collections. **`list`** is the most compact.

## Entity Views

`entity_views` declares the canonical detail view for each entity type — the
view that opens when a user clicks on an entity reference anywhere in the
data-entry app (a list row, a custom view's `display: list` section, a card,
a table cell). Without an entry, the SPA falls back to a generic
`/entity/<type>/<id>` page.

```yaml
entity_views:
  ticket:
    detail_view: ticket_detail
  decision:
    detail_view: decision_detail
```

### Fields

| Field         | Type   | Description                                                       |
| ------------- | ------ | ----------------------------------------------------------------- |
| `detail_view` | string | View name (must reference a key under `views:`) used for entities of this type |

### How navigation resolves

For each clickable entity reference, the SPA picks the destination URL using
the following priority:

1. A column-level `link:` on a list (server-resolved, e.g. `link: detail` or
   `link: document/<name>`).
2. `entity_views.<type>.detail_view` (the canonical detail view for the
   type) → `/view/<viewId>/<id>`.
3. Fallback: `/entity/<type>/<id>` (a generic detail page).

This means you only configure the destination *once* per entity type, and
every consumer (lists, view sections, table rows) routes consistently.

### Migration from list-level `detail_view`

Earlier versions accepted `detail_view` directly on each list. That field is
now deprecated; the canonical home is `entity_views.<type>.detail_view`. Run
`rela migrate` to lift existing list-level values into the new section
automatically. If two lists for the same entity type set conflicting
`detail_view` values, the migration leaves them in place — resolve the
conflict by hand and run `rela migrate` again.

## Dashboard

The dashboard is an optional overview page that displays a grid of query-driven cards. Each card
runs a search query against your entities and renders the results as a count, a property breakdown,
or a mini-table. The query syntax is the same as the search page: `type:`, `prop:`, `status:`,
and free text.

### Basic Dashboard

```yaml
dashboard:
  title: "Dashboard"
  description: "Project overview"
  cards:
    - title: "Open Tickets"
      query: "type:ticket status:open"
      display: count

    - title: "By Priority"
      query: "type:ticket"
      display: breakdown
      group_by: priority

    - title: "Critical Issues"
      query: "type:ticket prop:priority=critical"
      display: table
      columns:
        - property: title
          label: "Title"
          link: true
        - property: status
          label: "Status"
        - property: assignee
          label: "Assignee"
      sort:
        property: status
        direction: asc
      limit: 10
```

### Dashboard Fields

| Field         | Type   | Description                            |
| ------------- | ------ | -------------------------------------- |
| `title`       | string | Page heading                           |
| `description` | string | Subtitle shown below the heading       |
| `cards`       | list   | Card definitions                       |

### Card Options

| Field     | Type   | Description                                                        |
| --------- | ------ | ------------------------------------------------------------------ |
| `title`   | string | Card heading                                                       |
| `query`   | string | Search query (same syntax as the search page)                      |
| `display` | string | Display mode: `"count"`, `"breakdown"`, or `"table"`               |
| `group_by`| string | Property to group by (`breakdown` mode only)                       |
| `columns` | list   | Column definitions (`table` mode only, same format as list columns) |
| `sort`    | object | Sort order (`table` mode only)                                     |
| `limit`   | int    | Maximum rows to display (`table` mode only)                        |

### Display Modes

**`count`** shows a single large number — the count of entities matching the query. Use this for
quick status indicators like "5 open tickets" or "3 overdue items".

**`breakdown`** groups matching entities by a property and shows each value with a count and a
horizontal bar. The property should be an enum or custom type so values can be styled with badge
colors from `styles`. For example, grouping by `status` shows "open: 2, in-progress: 1, closed: 1"
with colored bars.

**`table`** shows matching entities as a compact table. It supports the same `columns` format as
list definitions (with `property`, `label`, `sortable`, `link`), plus `sort` and `limit` to control
ordering and row count.

### Query Syntax

Cards use the same search query syntax available on the search page:

| Syntax                   | Example                           | Description                      |
| ------------------------ | --------------------------------- | -------------------------------- |
| `type:<entity_type>`     | `type:ticket`                     | Filter by entity type            |
| `type:<a>,<b>`           | `type:ticket,category`            | Multiple entity types            |
| `status:<value>`         | `status:open`                     | Shortcut for `prop:status=value` |
| `prop:<name>=<value>`    | `prop:priority=critical`          | Property equals value            |
| `prop:<name>!=<value>`   | `prop:assignee!=`                 | Property not equal               |
| `prop:<name>=~<regex>`   | `prop:title=~auth.*`              | Regex match                      |
| `prop:<name><<value>`    | `prop:due_date<2025-06-01`        | Less than (dates, numbers)       |
| `sort:<property>`        | `sort:priority`                   | Sort ascending by property       |
| `sort:<property>:desc`   | `sort:priority:desc`              | Sort descending by property      |
| `sort:id` / `sort:modified` | `sort:modified:desc`           | Sort by ID or modification time  |
| free text                | `authentication`                  | Substring match across all fields|
| `"quoted phrase"`        | `"REST API"`                      | Exact phrase match               |

Multiple terms are combined with AND logic. For example,
`type:ticket status:open prop:priority=critical` matches tickets that are both open and critical.

Every card includes a link icon that opens the same query on the search page for further
exploration.

## Kanbans

Kanbans provide a visual board view where entities are displayed as cards grouped into columns
(and optionally swimlanes). Cards can be dragged between columns/swimlanes to update the
underlying entity properties.

### Basic Kanban

```yaml
kanbans:
  ticket_board:
    entity_type: ticket
    title: "Ticket Board"
    column_property: status
    card:
      title: title
      fields:
        - property: priority
        - property: assignee
    edit_form: edit_ticket
    create_form: create_ticket
```

### Kanban Fields

| Field              | Type   | Description                                                |
| ------------------ | ------ | ---------------------------------------------------------- |
| `entity_type`      | string | Entity type to display on the board                        |
| `title`            | string | Board heading                                              |
| `column_property`  | string | Property to group by for columns (must be enum/custom type)|
| `columns`          | list   | Explicit column definitions (optional)                     |
| `swimlane_property`| string | Property to group by for swimlanes (optional)              |
| `swimlanes`        | list   | Explicit swimlane definitions (optional)                   |
| `card`             | object | Card display configuration                                 |
| `edit_form`        | string | Form name for editing cards (click to open)                |
| `create_form`      | string | Form name for the "New" button                             |
| `filters`          | list   | Static filters (same as lists)                             |
| `filter_controls`  | list   | Interactive filter controls (same as lists)                |

### Columns

By default, columns are inferred from the enum values of `column_property` in the metamodel.
To customize column order or labels, define explicit columns:

```yaml
kanbans:
  ticket_board:
    entity_type: ticket
    column_property: status
    columns:
      - value: open
        label: "📥 To Do"
      - value: in-progress
        label: "🔧 In Progress"
      - value: resolved
        label: "✅ Done"
```

| Field   | Type   | Description                                    |
| ------- | ------ | ---------------------------------------------- |
| `value` | string | Enum value that maps to this column            |
| `label` | string | Display label (defaults to title-cased value)  |

Entities with column property values not in the explicit list are hidden from the board.

### Swimlanes

Add a second grouping dimension with swimlanes. This creates a grid where columns are horizontal
and swimlanes are vertical rows:

```yaml
kanbans:
  priority_board:
    entity_type: ticket
    column_property: priority
    swimlane_property: status
    swimlanes:
      - value: open
      - value: in-progress
      - value: resolved
```

| Field   | Type   | Description                                      |
| ------- | ------ | ------------------------------------------------ |
| `value` | string | Enum value that maps to this swimlane            |
| `label` | string | Display label (defaults to title-cased value)    |

Without explicit swimlanes, values are inferred from the metamodel. Entities whose swimlane
property value is not in the list are hidden.

### Card Configuration

The `card` object controls what's displayed on each card:

```yaml
card:
  title: title          # Property to use as card heading
  fields:               # Additional fields shown on the card
    - property: priority
    - property: assignee
      label: "Owner"
```

| Field    | Type   | Description                                           |
| -------- | ------ | ----------------------------------------------------- |
| `title`  | string | Property name for the card heading                    |
| `fields` | list   | Additional properties displayed as badges on the card |

Card fields use the same styling as lists — enum values are displayed with colors from `styles`.

### Drag and Drop

Cards can be dragged between columns (and swimlanes if configured). Dropping a card updates
the entity's column property (and swimlane property) and saves the change to disk. The board
re-renders to reflect the new state.

### Navigation

Add kanban boards to the sidebar using the `kanban` field in navigation entries:

```yaml
navigation:
  - group: "Boards"
    items:
      - label: "Ticket Board"
        kanban: ticket_board
      - label: "Priority Board"
        kanban: priority_board
```

### Keyboard Shortcuts

| Key | Action                              |
| --- | ----------------------------------- |
| `N` | Open the create form (if configured)|

### Complete Example

```yaml
kanbans:
  ticket_board:
    entity_type: ticket
    title: "Ticket Board"
    column_property: status
    columns:
      - value: open
        label: "📥 To Do"
      - value: in-progress
        label: "🔧 In Progress"
      - value: resolved
        label: "✅ Done"
    card:
      title: title
      fields:
        - property: priority
        - property: assignee
    edit_form: edit_ticket
    create_form: create_ticket
    filter_controls:
      - property: priority
        widget: select

  priority_board:
    entity_type: ticket
    title: "Priority Board"
    column_property: priority
    swimlane_property: status
    swimlanes:
      - value: open
      - value: in-progress
      - value: resolved
    card:
      title: title
      fields:
        - property: assignee
    edit_form: edit_ticket
    create_form: create_ticket
    filters:
      - property: status
        operator: "!="
        value: closed
```

## Navigation

The navigation section defines the sidebar menu. Each entry is either a direct item (linking to a
list, dashboard, or graph) or a **group** containing multiple items:

```yaml
navigation:
  - label: "Dashboard"
    dashboard: true
  - group: "Tickets"
    items:
      - label: "Open Tickets"
        list: open_tickets
      - label: "All Tickets"
        list: all_tickets
  - group: "Reference Data"
    collapsed: true
    items:
      - label: "Categories"
        list: categories
  - label: "Graph Explorer"
    graph: true
```

### Direct Items

| Field       | Type   | Description                                                    |
| ----------- | ------ | -------------------------------------------------------------- |
| `label`     | string | Menu item text                                                 |
| `list`      | string | List name to navigate to (mutually exclusive with other types) |
| `kanban`    | string | Kanban board name to navigate to                               |
| `dashboard` | bool   | Link to the dashboard page                                     |
| `graph`     | bool   | Link to the graph explorer                                     |
| `action`    | string | Action ID to trigger when clicked (renders as a sidebar button)|

### Groups

| Field       | Type   | Description                                              |
| ----------- | ------ | -------------------------------------------------------- |
| `group`     | string | Group header text (displayed as uppercase label)         |
| `collapsed` | bool   | Default collapsed state (optional, default: `false`)     |
| `items`     | list   | List of direct navigation items within the group         |

Groups appear as collapsible sections in the sidebar. Clicking the group header toggles
expand/collapse. The collapsed state is persisted server-side in `.rela/ui-state.json`, so it
survives page reloads. If the active page is inside a collapsed group, the group auto-expands.

Nested groups are not supported. If an item inside `items` has a `group` field, config validation
will reject it with a clear error message.

The first navigable entry is the default landing page — the first direct item, or the first item
inside the first group. Order matters; items appear in the sidebar in the order listed.

List entries show an entity count badge next to the label (based on the list's filters). Dashboard
and graph entries do not show a count.

Direct items and groups can be freely mixed in any order.

## Actions

Actions define quick operations that can be triggered from list views or the sidebar. An action
either mutates entity properties declaratively (`set`) or runs a Lua script (`script`).

### Defining Actions

Actions are defined at the top level of `data-entry.yaml`:

```yaml
actions:
  resolve-ticket:
    label: "Resolve"
    key: "r"
    set:
      status: resolved

  close-ticket:
    label: "Close"
    key: "c"
    confirm: true
    set:
      status: closed

  run-checks:
    label: "Validate"
    key: "v"
    script: validate-ticket.lua
    params:
      strict: "true"
```

### Action Fields

| Field         | Type   | Description                                                     |
| ------------- | ------ | --------------------------------------------------------------- |
| `label`       | string | Display text (required when referenced by a list)               |
| `key`         | string | Keyboard shortcut — single lowercase letter or digit (required when referenced by a list) |
| `description` | string | Optional description                                            |
| `set`         | map    | Property key-value pairs to set on the entity (mutually exclusive with `script`) |
| `script`      | string | Lua script path, relative to the `actions/` directory (mutually exclusive with `set`) |
| `params`      | map    | Static key-value parameters from config, exposed as `rela.params` (values must be strings — quote them in YAML) |
| `confirm`     | bool   | Show a confirmation dialog before executing (default: `false`)  |

Each action must have either `set` or `script`, not both.

`params` is **static config**, not runtime context: the values come from
`data-entry.yaml` and are the same for every invocation. The selected
entity (for list actions) is exposed separately via the `entity` global —
see [Lua Action Scripts](#lua-action-scripts).

### Using Actions in Lists

Reference action IDs in a list's `actions` field to make them available as keyboard shortcuts
on selected rows:

```yaml
lists:
  all_tickets:
    entity_type: ticket
    title: "All Tickets"
    columns:
      - property: title
        link: true
      - property: status
    actions: [resolve-ticket, close-ticket]
```

When a list has actions, the configured keyboard shortcuts appear in the list's toolbar.
Select one or more rows, then press the shortcut key to apply the action to all selected entities.

### Using Actions in Navigation

Reference an action ID in a navigation entry to render it as a sidebar button:

```yaml
navigation:
  - label: "Run Checks"
    action: run-checks
```

When clicked, the action executes. If the action script returns a `redirect`, the UI navigates
to that path. If it returns a `message`, a toast notification is shown.

### Lua Action Scripts

Action scripts live in the `actions/` directory at the project root. They have full access
to the rela Lua API (entity CRUD, graph queries, AI). A script can optionally return a table
to control the UI response.

#### Inputs available to the script

| Source        | Where           | Populated when                                                              |
| ------------- | --------------- | --------------------------------------------------------------------------- |
| Static config | `rela.params`   | Always — values from the action's `params:` map in `data-entry.yaml`        |
| Selected row  | `entity` global | Only when invoked from a list against a selected row (one call per entity). The table has `id`, `type`, `properties`, `content`, `mod_time`, plus `prop(name, default)` and `strip_prefix()` methods |

When invoked from a navigation sidebar button, no entity is selected — the
`entity` global is `nil`. Always nil-check it.

```lua
-- actions/validate-ticket.lua
-- Selected row from the list (nil for sidebar/nav invocations).
if entity == nil then
    return { message = "Select a ticket first", message_type = "warning" }
end

-- Static parameter from data-entry.yaml — values are always strings.
local strict = rela.params.strict == "true"

-- ... perform validation against entity.id, entity.properties, ... ...

return {
    message = "Validation passed",
    message_type = "success",      -- "success", "info", "warning", or "error"
    redirect = "/list/all_tickets" -- optional: navigate after completion
}
```

Scripts have a 5-second execution timeout (tighter than the default Lua
timeout because the action handler holds a global write lock for the
duration — concurrent mutations and other actions wait). Returning
nothing (or `nil`) produces a silent success response.

### Reserved Keyboard Shortcuts

The following keys are reserved for built-in list navigation and cannot be used as action keys:

| Key | Built-in Function |
| --- | ----------------- |
| `j` | Move selection down |
| `k` | Move selection up |
| `o` | Open selected entity |
| `e` | Edit selected entity |
| `n` | Create new entity |
| `h` | Previous page |
| `l` | Next page |

### Validation Rules

- Action IDs must match `^[a-z0-9_-]{1,64}$`
- `set` properties must exist in the entity type's metamodel
- `script` paths must end in `.lua` and be local paths (no `..` or absolute paths)
- Keys must be unique within a list (no two actions on the same list can share a key)

## Commands

Commands let you define shell scripts in `data-entry.yaml` that users can trigger from the UI.
Each command receives context-specific JSON on stdin and streams results back to the browser
as toast notifications using the `::rela::` line protocol.

### Configuration

Define commands under the `commands:` key:

```yaml
commands:
  export-json:
    label: "Export JSON"
    script: |
      echo '::rela::{"type":"message","text":"Exporting..."}'
      jq '.' > /tmp/export.json
      echo '::rela::{"type":"file","path":"/tmp/export.json","label":"JSON Export","action":"reveal"}'
    context: entity
    available_on:
      entity_types: [ticket]
    confirm: "Export this entity?"
    env:
      FORMAT: json
```

| Field          | Type   | Description                                            |
| -------------- | ------ | ------------------------------------------------------ |
| `label`        | string | Button text shown in the UI (required)                 |
| `script`       | string | Shell script executed via `sh -c` (required)           |
| `context`      | string | Scope: `entity`, `list`, `view`, or `global` (required)|
| `available_on` | object | Restrict where the button appears (optional)           |
| `confirm`      | string | Confirmation prompt before execution (optional)        |
| `env`          | map    | Custom environment variables (optional)                |
| `auto_open`    | bool   | Auto-open output files on completion (optional)        |

### Context Scopes

Each command runs in one of four scopes, which determines the JSON it receives on stdin:

**`entity`** — runs from entity detail and view pages. Receives the entity with all properties,
content, and relations.

**`list`** — runs from list pages. Receives all entities currently visible in the list (after
filters).

**`view`** — runs from view pages only. Receives the entry entity, all traversed collections,
and relations between them.

**`global`** — runs from the dashboard. Receives only project metadata.

### Visibility Rules (`available_on`)

Without `available_on`, a command appears on every page that matches its context. Add
`available_on` to restrict it:

```yaml
available_on:
  views: [ticket_report]      # Only on specific views
  lists: [all_tickets]         # Only on specific lists
  entity_types: [ticket]       # Only for specific entity types
  dashboard: true              # Only on the dashboard (global context)
```

A command appears if **any** condition matches.

### Environment Variables

Commands always receive:

| Variable            | Description                              |
| ------------------- | ---------------------------------------- |
| `RELA_PROJECT_ROOT` | Absolute path to the project root        |
| `RELA_CONTEXT`      | Context type (`entity`/`list`/`view`/`global`) |

Context-specific variables:

| Variable            | Available In         | Description              |
| ------------------- | -------------------- | ------------------------ |
| `RELA_ENTITY_ID`    | entity, view         | Current entity ID        |
| `RELA_ENTITY_TYPE`  | entity, view         | Current entity type      |
| `RELA_LIST_ID`      | list                 | Current list ID          |
| `RELA_VIEW_ID`      | view                 | Current view ID          |

Custom variables from `env:` are added to the process environment.

### The `::rela::` Line Protocol

Commands communicate results by writing lines to stdout with a `::rela::` prefix followed by
JSON. Lines without the prefix are treated as log output.

**Message types:**

| Type       | Purpose                          | Key Fields                            |
| ---------- | -------------------------------- | ------------------------------------- |
| `message`  | Toast notification               | `text`, `level` (info/warning/error)  |
| `error`    | Error toast                      | `text`                                |
| `file`     | Open or reveal a file            | `path`, `label`, `action` (open/reveal) |
| `entity`   | Entity update notification       | `id`, `entity_type`, `action` (created/updated/deleted) |
| `open`     | Open URL in browser              | `url`                                 |
| `group`    | Start a collapsible group        | `label`                               |
| `endgroup` | End the current group            | —                                     |

**Example script:**

```bash
echo '::rela::{"type":"group","label":"Generated Files"}'
echo '::rela::{"type":"file","path":"/tmp/report.pdf","label":"PDF Report","action":"open"}'
echo '::rela::{"type":"file","path":"/tmp/data.csv","label":"CSV Data","action":"reveal"}'
echo '::rela::{"type":"endgroup"}'
echo '::rela::{"type":"message","text":"Done!","level":"info"}'
```

### Auto-Open

When `auto_open: true` is set on a command, all output files with `action: "open"` are
automatically opened when the command completes successfully, and the toast is dismissed.
This is useful for commands that produce a single output file where the extra click to
open it would be redundant:

```yaml
commands:
  generate-pdf:
    label: "Generate PDF"
    script: |
      PDF="/tmp/report-${RELA_ENTITY_ID}.pdf"
      # ... generate PDF ...
      echo "::rela::{\"type\":\"file\",\"path\":\"$PDF\",\"label\":\"Report\",\"action\":\"open\"}"
    context: entity
    auto_open: true
```

If the command fails or no files have `action: "open"`, the toast stays visible with
the normal interactive buttons.

### Streaming and Cancellation

Command output streams in real time into stacked toast notifications. Long-running commands
can be cancelled by the user via a cancel button.

## User Defaults

The data entry app includes a **Settings** page where users can configure default values for
properties and relations. These defaults are applied automatically when creating new entities,
reducing repetitive data entry.

### Storage

User defaults are stored in `.rela/user-defaults.yaml` (gitignored, per-user). The file is
created automatically when a user saves settings for the first time.

```yaml
# .rela/user-defaults.yaml
defaults:
    assignee: alice
    priority: high
relations:
    belongs-to: backend
overrides:
    - entity_types:
        - ticket
      defaults:
          reporter: bob
      relations:
          tagged: bug
```

### Settings Page

The Settings page is accessible from the sidebar (gear icon at the bottom). It has three sections:

**Property Defaults** — Set default values for any property defined in the metamodel. The widget
type (text input, dropdown, date picker, etc.) matches the property's type. For enum/custom types,
a dropdown shows the allowed values.

**Relation Defaults** — Set a default target entity for any relation type. When creating a new
entity, the relation will be pre-filled with this target.

**Overrides** — Scope defaults to specific entity types. For example, set `priority: critical`
only when creating tickets, while leaving the global default as `medium`.

### Resolution Order

When creating a new entity, default values are resolved in this order (highest priority first):

1. **Entity-type override** from user defaults (e.g., ticket-specific override)
2. **Global user default** (e.g., `assignee: alice`)
3. **Form-level default** (from `data-entry.yaml`, e.g., `default: medium`)
4. **Metamodel default** (from `metamodel.yaml` type definition)

User defaults never override values explicitly set by the user in the form.

## Complete Example

A ticket management system with forms, lists, views, dashboard, and grouped navigation:

```yaml
version: "1.0"

app:
  name: "Support Tickets"
  description: "Internal ticket management"

git:
  require_pr: [main]

styles:
  ticket_status:
    open: blue
    in-progress: purple
    resolved: green
    closed: gray
  priority:
    critical: red
    high: orange
    medium: yellow
    low: green

forms:
  create_ticket:
    entity_type: ticket
    title: "New Ticket"
    body: true
    fields:
      - property: title
        label: "Title"
        required: true
      - property: priority
        label: "Priority"
        default: medium
      - property: assignee
        label: "Assignee"
      - property: due_date
        label: "Due Date"
        widget: date
      - property: status
        hidden: true
        default: open
    relations:
      - relation: belongs-to
        direction: outgoing
        target_type: category
        label: "Category"
        widget: select
        allow_create: true
        create_form: create_category

  edit_ticket:
    entity_type: ticket
    title: "Edit Ticket"
    mode: edit
    body: true
    fields:
      - property: title
        label: "Title"
      - property: status
        label: "Status"
        transitions:
          open: [in-progress, closed]
          in-progress: [open, resolved]
          resolved: [closed, in-progress]
          closed: [open]
      - property: priority
        label: "Priority"
      - property: assignee
        label: "Assignee"
      - property: due_date
        label: "Due Date"
        widget: date

  create_category:
    entity_type: category
    title: "New Category"
    fields:
      - property: name
        label: "Name"
        required: true

actions:
  resolve-ticket:
    label: "Resolve"
    key: "r"
    set:
      status: resolved
  close-ticket:
    label: "Close"
    key: "c"
    confirm: true
    set:
      status: closed

lists:
  all_tickets:
    entity_type: ticket
    title: "All Tickets"
    columns:
      - property: title
        label: "Title"
        sortable: true
        link: true
      - property: status
        label: "Status"
        sortable: true
      - property: priority
        label: "Priority"
        sortable: true
      - property: assignee
        label: "Assignee"
      - property: due_date
        label: "Due"
        sortable: true
    sort:
      property: priority
      direction: asc
    filter_controls:
      - property: status
        widget: multi-select
      - property: priority
        widget: select
    create_form: create_ticket
    edit_form: edit_ticket
    actions: [resolve-ticket, close-ticket]
    page_size: 25

  open_tickets:
    entity_type: ticket
    title: "Open Tickets"
    columns:
      - property: title
        link: true
        sortable: true
      - property: priority
        sortable: true
      - property: assignee
    filters:
      - property: status
        operator: "="
        value: open
    create_form: create_ticket
    edit_form: edit_ticket
    page_size: 25

  my_tickets:
    entity_type: ticket
    title: "My Tickets"
    columns:
      - property: title
        link: true
        sortable: true
      - property: status
        sortable: true
      - property: priority
        sortable: true
    filters:
      - property: assignee
        operator: "="
        value: "$USER"
    create_form: create_ticket
    edit_form: edit_ticket
    page_size: 25

entity_views:
  ticket:
    detail_view: ticket_detail

views:
  ticket_detail:
    title: "Ticket Detail"
    entry:
      type: ticket
    traverse:
      - from: entry
        follow: blocks
        collect_as: blocks
      - from: entry
        follow_incoming: blocks
        collect_as: blocked_by
    sections:
      - heading: "Ticket"
        source: entry
        display: properties
        fields:
          - property: status
          - property: priority
          - property: assignee
          - property: due_date
            label: "Due Date"
      - source: entry
        display: content
      - heading: "Blocks"
        source: blocks
        display: cards
        fields:
          - property: status
          - property: priority
        empty_message: "Not blocking any tickets"
      - heading: "Blocked By"
        source: blocked_by
        display: cards
        fields:
          - property: status
        empty_message: "Not blocked"

dashboard:
  title: "Dashboard"
  description: "Ticket overview"
  cards:
    - title: "Open Tickets"
      query: "type:ticket status:open"
      display: count
    - title: "By Status"
      query: "type:ticket"
      display: breakdown
      group_by: ticket_status
    - title: "Critical"
      query: "type:ticket prop:priority=critical"
      display: table
      columns:
        - property: title
          label: "Title"
          link: true
        - property: assignee
          label: "Assignee"
      limit: 5

commands:
  generate-pdf:
    label: "Generate PDF"
    script: |
      PDF="/tmp/ticket-${RELA_ENTITY_ID}.pdf"
      # ... generate PDF ...
      echo "::rela::{\"type\":\"file\",\"path\":\"$PDF\",\"label\":\"Ticket PDF\",\"action\":\"open\"}"
    context: entity
    auto_open: true
    available_on:
      entity_types: [ticket]

kanbans:
  ticket_board:
    entity_type: ticket
    title: "Ticket Board"
    column_property: ticket_status
    columns:
      - value: open
        label: "📥 To Do"
      - value: in-progress
        label: "🔧 In Progress"
      - value: resolved
        label: "✅ Done"
    card:
      title: title
      fields:
        - property: priority
        - property: assignee
    edit_form: edit_ticket
    create_form: create_ticket

navigation:
  - label: "Dashboard"
    dashboard: true
  - group: "Tickets"
    items:
      - label: "My Tickets"
        list: my_tickets
      - label: "Open Tickets"
        list: open_tickets
      - label: "All Tickets"
        list: all_tickets
      - label: "Ticket Board"
        kanban: ticket_board
```

## Analysis

The data entry app includes a built-in analysis page at `/analyze` that runs the same quality
checks as the CLI's `rela analyze all` command. It checks properties, cardinality constraints,
custom validations, orphans, duplicates, and ID gaps — displaying results grouped by category
with links to affected entities.

When a dashboard is configured, a validation summary card is automatically appended showing the
total error and warning counts with a link to the full analysis page.

No configuration is needed — the analysis page is always available in the sidebar.

## Documents

Documents are read-only rendered markdown panels attached to an entity's detail
view. A document's configuration declares which entity type it applies to and
how to produce the markdown — either a shell `command:` that writes markdown to
stdout, or a Lua `script:` that does the same via the embedded runtime.
Captured markdown is converted to HTML via goldmark. Links using
app-relative paths (e.g. `/form/<form_id>/<entity_id>`, `/entity/ticket/TKT-001`)
get a `return_to` query param appended automatically on form links so the
user lands back on the document after submitting the form. See "Links in
rendered documents" below.

The frontend's `DocumentsPanel.vue` shows every document whose `entity_type`
matches the current entity. SSE live-reload re-renders a document when the
entity changes (see the "SSE live-reload" caveat below).

A document is also reachable on its own page at
`/document/<name>/<entity_id>` (used by `rela.url.document` links and direct
deep-links). On that page the header shows Back and Refresh by default; add
an `edit:` block to the doc config to also expose an Edit button that takes
the user to a configured form, with a `return_to` query param so saving
returns to the document.

### YAML schema

```yaml
documents:
  release_notes:
    title: "Release Notes"         # shown as the panel title
    entity_type: release           # REQUIRED; renderer runs only for this type
    script: docs/release_notes.lua # OR command: — exactly one must be set
    timeout: 15                    # seconds; defaults to 30
    edit:                          # optional; renders an Edit button on the
      form: edit_release           # standalone /document/... page
      label: "Edit release"
  ticket_summary:
    title: "Ticket Summary"
    entity_type: ticket
    command: "my-renderer {id}"    # {id} / {id_lower} are substituted
    timeout: 30
```

Validation is strict: `entity_type:` must be set, and exactly one of
`command:` or `script:` must be non-empty. Configs with both, or with
neither, are rejected at startup. For `script:` docs, the referenced file
is checked for existence at startup too, so typos fail loudly instead of at
the first HTTP request. When an `edit:` block is present, both `form:` and
`label:` are required and `form:` must reference a known form ID. Note that a
bare `edit:` line with no subkeys is treated as "field absent" (no button, no
validation error); to catch a stub block write `edit: {}` instead so the
required-field checks fire.

### Shell command renderer (`command:`)

The command runs in a POSIX shell (`sh -c`) with the project root as the
working directory. The script must write the rendered markdown to stdout.
Placeholders inside the command string are substituted before execution:

| Placeholder | Expands to |
|-------------|------------|
| `{id}`      | The entry entity ID |
| `{id_lower}`| The ID lowercased |

Command renderer output is cached to disk at
`.rela/documents/<entry>-<hash>.html` keyed by an FNV hash of the entry
entity. Subsequent requests for the same entity skip the command and serve
the cached HTML until the entity changes.

### Lua script renderer (`script:`)

The `script:` field is a path under the project's `scripts/` directory
(e.g. `docs/release_notes.lua`). The script runs with a writer runtime —
it can read the full graph, call `ai.chat`, and use `rela.cache.memoize` —
but anything it writes to stdout (via `print()`) is captured as the
document's markdown.

When invoked in document mode, the runtime exposes extra context:

| Variable                   | Meaning |
|----------------------------|---------|
| `rela.mode`                | Always `"document"` in this context; `nil` elsewhere |
| `rela.document.id`         | The key under `documents:` in `data-entry.yaml` |
| `rela.document.entry_id`   | The ID of the entity being rendered |

Example — a document that composes a markdown doc from an entity plus its
graph neighbours:

```lua
-- scripts/docs/release_notes.lua
local entry = rela.get_entity(rela.document.entry_id)
print("# " .. (entry.properties.title or entry.id))
print()
for _, child in ipairs((rela.trace_from(entry.id, 2) or {children = {}}).children) do
  local e = rela.get_entity(child.id)
  if e then
    -- rela.url.form_edit builds an edit-form URL; rela.url.detail
    -- would be an alternative that links to the canonical detail page.
    -- rela.md.link emits [text](url) so we don't hand-concatenate markdown.
    local href = rela.url.form_edit("full_ticket", e)
    print("## " .. rela.md.link(e.id, href))
    print(e.content or "")
  end
end
```

Lua renders bypass the disk cache. In-process caching is available through
`rela.cache` (see [Lua Scripting → Cache](GUIDE-lua-scripting.md#cache) for
the full API). Memoized work is shared across HTTP requests within the
lifetime of the `rela-server` process. The cache namespace is the script
path, not the document's config key — shared helper scripts intentionally
share cache state across all documents that use them; if you need
doc-scoped keys, include `rela.document.id` in your cache key explicitly.

`rela.output({...})` in document mode emits a warning line into the
rendered document (captured stdout is the document body, so raw JSON in the
middle of markdown is almost always a mistake). If you need to output
data, use `print()`; if you're debugging, a warning line is usually fine.
A script that calls `rela.output` inside a loop will produce many warning
lines in the rendered output — that is intentionally loud.

### Links in rendered documents

Documents link to anywhere in the SPA by writing app-relative paths. The
goldmark→HTML step walks every `href="/..."` attribute and appends a
`return_to` query param. Every screen reachable from a document link
(entity detail, list, kanban, view, custom view, another document,
search, analyze) surfaces a "← Back" affordance — see
[Back navigation](#back-navigation) below.

| Target                | Write this in markdown                          | Notes                               |
|-----------------------|-------------------------------------------------|-------------------------------------|
| Edit an entity        | `[Edit](/form/full_ticket/TKT-001)`             | Adds `return_to=...`; stable `id="edit-tkt-001-<n>"` on the link for scroll-back |
| Create a new entity   | `[New](/form/full_ticket)`                      | Adds `return_to=...`; stable `id="create-full_ticket-<n>"` |
| Create with defaults  | `[New](/form/full_ticket?prop.status=open)`     | Preserves query + appends `return_to` |
| Link to entity detail | `[Detail](/entity/ticket/TKT-001)`              | Adds `return_to=...` — EntityView renders a Back button |
| Link to a list        | `[All](/list/all_tasks)`                        | Adds `return_to=...` — ListView renders a Back button |
| Link to a kanban      | `[Board](/kanban/sprint)`                       | Adds `return_to=...` — KanbanView renders a Back button |
| External link         | `[Docs](https://example.com)`                   | Untouched                           |

The rewriter is the single source of truth for `return_to` on emitted
HTML: any author-supplied `return_to` on an internal link is stripped
(with a warning) and replaced with one the server controls. The legacy
`edit://` / `create://` schemes log a warning and pass through unchanged
so downstream projects notice and migrate. Cached document renders
(`.rela/documents/<entry>-<hash>.html`) are `return_to`-free; the
rewrite happens after the cache read, so two viewers of the same entry
under different `return_to` values each get their own value rewritten
in.

### Back navigation

A view rendered from a document link shows a Back button that returns
the user to the source document. The button follows a simple
precedence:

1. `?return_to=<path>` — set by the rewriter. Validated by the
   open-redirect guard both server-side and client-side; unsafe values
   (protocol-relative `//evil.com`, percent-encoded separators,
   schemed URLs) are rejected silently.
2. `?from=<list-id>` — legacy affordance used by EntityView's scope
   navigation (Prev/Next through a list). When present, the Back
   button points to `/list/<id>` and is labelled `← <list title>` if
   the list is configured.
3. Neither — no Back button renders. The user navigated in directly
   (sidebar, bookmark), not from a back-able context.

Scope navigation (Prev/Next through a list) is independent of the Back
mechanism: if a user arrives at an entity via `?from=tasks&return_to=/doc`,
Back follows `return_to` (the document) while Prev/Next still walks the
`tasks` list and preserves `return_to` across in-list navigation.

### Building links from Lua: `rela.url`

Document scripts build URLs via the `rela.url` submodule. Each helper
corresponds to one route kind the SPA exposes. Helpers are pure string
builders — a typo in a form name produces a syntactically valid URL; the
404 surfaces in the SPA when the user clicks.

| Helper | Returns | Typical use |
|--------|---------|-------------|
| `rela.url.form_edit(name, entity)` | `/form/<name>/<entity.id>` | Edit-link for an entity, using form `<name>` |
| `rela.url.form_create(name, {relations, properties, query})` | `/form/<name>?…` | Create-link with pre-filled relations/properties |
| `rela.url.form_create(name)` | `/form/<name>` | Create-link with no pre-fill |
| `rela.url.detail(entity)` | `/entity/<entity.type>/<entity.id>` | Canonical entity detail page |
| `rela.url.list(name, query?)` | `/list/<name>?…` | Link to a configured list |
| `rela.url.view(name, entity)` | `/view/<name>/<entity.id>` | Custom view for an entity |
| `rela.url.kanban(name, query?)` | `/kanban/<name>?…` | Kanban board |
| `rela.url.document(name, entity)` | `/document/<name>/<entity.id>` | Render a different document for an entity |
| `rela.url.home(query?)` | `/dashboard?…` | App home |
| `rela.url.search(query?)` | `/search?…` | Full-text search |
| `rela.url.analyze(query?)` | `/analyze?…` | Graph analysis |
| `rela.url.settings(query?)` | `/settings?…` | App settings |
| `rela.url.conflicts(query?)` | `/conflicts?…` | Git conflicts |

Every frontend route has a typed helper. The `query?` parameter on
non-form helpers is an optional flat table of `{key = value}` pairs —
no `{query = {...}}` wrapping.

`form_edit` and `form_create` are split (not one overloaded `form(...)`) so
an author who writes `rela.url.form_create("x", {id = "prefill-x"})` meaning
"create with a prefilled id property" gets a create form — not silently
routed to edit mode on the basis of a structural check of the opts table.

`form_create`'s opts table keeps the three-sub-key shape (`relations`,
`properties`, `query`) because it has three distinct semantics — the
helper adds the `rel.` and `prop.` prefixes the form expects, and
`query` is for passthrough.

Examples:

```lua
local ticket = rela.get_entity("TKT-001")

-- Edit the ticket in the "full_ticket" form.
rela.url.form_edit("full_ticket", ticket)
-- → "/form/full_ticket/TKT-001"

-- Create a new ticket pre-filled with relations and properties. Relation
-- and property names are taken from the metamodel; the helper adds the
-- "rel." / "prop." prefixes the form expects.
rela.url.form_create("create_ticket", {
  relations  = {parent = ticket.id, assignee = "actor-me"},
  properties = {status = "open", priority = "high"},
})
-- → "/form/create_ticket?prop.priority=high&prop.status=open&rel.assignee=actor-me&rel.parent=TKT-001"

-- Canonical detail page — no form choice, always safe.
rela.url.detail(ticket)
-- → "/entity/ticket/TKT-001"

-- Singleton with a query param.
rela.url.search({q = "pseudoniem"})
-- → "/search?q=pseudoniem"
```

Form links get a `return_to` query parameter injected by the document
link rewriter so submitting the form returns the user to the document.
`return_to` is reserved — setting it in any helper's query table is
rejected with a Lua error.

#### Pre-filling a create form

`form_create` accepts three kinds of defaults in its opts table; each
maps to a query-param convention the create form reads on mount:

| Opts key     | Query form       | What the form does on mount                          |
|--------------|------------------|------------------------------------------------------|
| `relations`  | `rel.<name>=<id>` | Adds `<id>` to the named relation's targets          |
| `properties` | `prop.<name>=<v>` | Sets the property's initial value                    |
| `query`      | `<k>=<v>`         | Passed through verbatim (use for custom URL params)  |

The form applies these defaults only on initial mount; the user can
still edit or clear each field before submitting. Multiple values for
the same relation accumulate (call `form_create` with a list-shaped
value only if the metamodel permits multi-target for that relation).

```lua
-- A "+ Add sub-ticket" link that pre-selects the parent and puts the new
-- ticket straight into the correct category:
rela.url.form_create("create_ticket", {
  relations  = {parent = ticket.id, ["belongs-to"] = ticket.properties.category},
  properties = {priority = "medium", reporter = "actor-me"},
})
```

Defaults set via link query string are overlaid on top of the project's
`.rela/user-defaults.yaml` and metamodel-level defaults; the order is
covered in the **User defaults** section earlier in this guide.

### Caching and live-reload

- **Command renders** are cached on disk (`.rela/documents/<entry>-<hash>.html`).
  The hash includes only the entry entity, so the cache refreshes when the
  entry entity changes.
- **Script renders** are not cached on disk. Use `rela.cache.memoize` inside
  your script to share work across requests within the same server process.
- **SSE live-reload** refreshes a document when the entry entity changes.
  Multi-entity composition (a script that walks 20 related tickets) will
  only re-render when the **entry** entity changes, not the walked ones.
  The refresh button in the UI is the escape hatch. A follow-up ticket
  (TKT-E1FO1) tracks the fix for explicit dependency tracking.

### Security notes

- Document scripts run in the same sandbox as action scripts: no `io`, no
  `os`, no `debug`; file writes are confined to `output/` via
  `rela.write_file`. Treat Lua scripts as trusted code.
- The HTTP handler enforces `entity_type:` on every request: a document
  configured for `entity_type: release` cannot be rendered against a
  ticket, even if the caller bypasses the frontend filter.
- Rendered markdown uses goldmark's `html.WithUnsafe` — the frontend
  (DOMPurify) is the sanitization boundary. If you add another consumer of
  the rendered HTML (PDF export, copy-HTML button, etc.), it must add its
  own sanitization.

### Config hot-reload

Editing `data-entry.yaml` to change a document's `script:` or `command:`
takes effect on the next request; open document panels pick up the new
renderer on their next reload.

## Custom apps

Custom **apps** let you extend the data-entry web app with your own HTML+JS
applications — dashboards, specialized forms, domain mini-tools — without
forking the SPA or writing Go. An app runs inside a locked-down sandboxed
iframe and talks to the existing REST API through a host-managed
`MessageChannel` bridge, so **an app can only ever do what the logged-in user
can already do**.

### Authoring

An app is a **folder** under the project's `apps/` directory (alongside
`actions/`, `scripts/`, `templates/`) containing an `index.html`. There is no
separate config:

```text
apps/ticket-counter/
  index.html          →  /app/ticket-counter   (id = folder name)
  app.js              (any sibling files: js, css, images, fonts…)
  style.css
```

The **id** is the folder name and must match `^[a-z0-9_-]{1,64}$`. A folder is
a live app iff it contains `index.html`. The app self-describes via optional
`<meta>` tags in `index.html`'s `<head>`:

```html
<head>
  <meta name="rela-app:label" content="Ticket Counter">
  <meta name="rela-app:title" content="Ticket Counter">
  <meta name="rela-app:description" content="Counts tickets by status">
  <!-- the bridge SDK (window.rela); served at the app's own path -->
  <script src="_rela.js"></script>
</head>
```

`label` (falling back to `title`, then the id) is the sidebar entry; the rest
are cosmetic. **The app must include `<script src="_rela.js"></script>`** to
get the `rela.*` bridge — rela serves it at the app's own `_rela.js` path.

The app and its files are served from `/api/v1/_apps/<id>/`, so reference
sibling assets with **relative** URLs (`<script src="app.js">`, `<img
src="logo.png">`).

**Publish / unpublish.** A folder with an `index.html` is live. To take an app
offline without deleting it, rename the folder (e.g. `ticket-counter` →
`_ticket-counter`, which fails the id rule) or remove its `index.html`.

### Matching rela's look (optional `_rela.css`)

To render consistently with the rest of the app, opt into rela's styling by
linking the served stylesheet:

```html
<head>
  <link rel="stylesheet" href="_rela.css">
</head>
```

`_rela.css` provides two things:

- **Theme tokens** — CSS custom properties for colors (`--text-color`,
  `--bg-color`, `--card-bg`, `--border-color`, `--accent-color`,
  `--error/success/warning/info-color`, the `--badge-*` set), surfaces, and
  borders. Use them in your own CSS (`color: var(--text-color)`) so the app
  matches the host palette.
- **Base controls** — three atomic classes: `.btn` / `.btn-primary` (buttons),
  `.input` (text inputs), `.card` (a bordered surface). These are deliberately
  minimal; build anything more structural (tables, selects, modals) yourself
  using the tokens.

**Dark mode follows the host automatically** — when the user switches the data-
entry theme, rela toggles the same `dark` class on your app, and the tokens
flip. No work needed beyond linking `_rela.css` and using `var(--…)` for your
own colors. Opting in is entirely optional; an app that wants full control of
its look simply doesn't link it.

### The `rela` bridge

Inside the iframe, a `rela` object (from `_rela.js`) gives the app a
promise-based, closed set of methods that forward to the REST API over the
`MessageChannel`:

| Method | REST operation |
|---|---|
| `rela.schema()` / `rela.config()` | metamodel + data-entry config |
| `rela.list({type, params})` | list entities of a type |
| `rela.get({type, id, params})` | fetch one entity |
| `rela.search({query, type})` | full-text search |
| `rela.analyze()` | run the analysis checks |
| `rela.templates({type})` | entity templates |
| `rela.position({id, scope})` | prev/next within an ordered set |
| `rela.create({type, entity})` | create an entity |
| `rela.update({type, id, patch, etag})` | update an entity |
| `rela.delete({type, id})` | delete an entity |
| `rela.relationCreate({type, id, relation, targetId, meta?, direction?})` | link entities |
| `rela.relationUpdate({type, id, relation, targetId, meta, direction?})` | edit a relation's properties |
| `rela.relationDelete({type, id, relation, targetId, direction?})` | unlink entities |
| `rela.action({actionId, entityId?, entityType?})` | run a registered Lua action |

This is a **closed set** — an app cannot ask the host to fetch an arbitrary
URL. Reads are scoped to the user's read permissions; writes go through the
normal write path (re-authorized and audited). A denied call rejects with an
error the app can catch. The SDK signals readiness with a one-time
`rela:ready` event; calls made before then are queued.

Minimal app (`apps/hello/index.html`):

```html
<!doctype html>
<html>
  <head><script src="_rela.js"></script></head>
  <body>
    <div id="out">loading…</div>
    <script>
      window.addEventListener('rela:ready', async () => {
        const res = await rela.list({ type: 'ticket', params: { per_page: 200 } });
        document.getElementById('out').textContent = res.data.length + ' tickets';
      });
    </script>
  </body>
</html>
```

### Security model

Apps run untrusted code, so the data-entry server and SPA lock them down:

- **Sandboxed iframe** — `sandbox="allow-scripts allow-forms"`, never
  `allow-same-origin`. No `localStorage`/parent-DOM access. The app loads from
  its own served path (`/api/v1/_apps/<id>/`) so its files resolve, which makes
  it same-origin with the API — so its confinement is the CSP, not the origin.
- **Path-scoped Content-Security-Policy (header)** — every resource directive
  is scoped to the app's **own** absolute subpath (e.g. `script-src
  https://host/api/v1/_apps/<id>/`), **not** `'self'` (which would include
  `/api/`, letting `<img src="/api/v1/tickets/x">` pull data). `connect-src
  'none'` means the app's own JS cannot `fetch`/`XHR`/WebSocket anything — so it
  **cannot reach `/api/` directly**. `form-action 'none'` + the sandbox block
  form/navigation exfil.
- **Bridge-only data path** — with `connect-src 'none'`, the only route to the
  API is the `MessageChannel` bridge (a message post, not a network request,
  so CSP doesn't block it). The bridge exposes only the closed method set above
  and is the per-app capability chokepoint.
- **No app-specific permissions (yet)** — an app is a UI shell. Its reads and
  writes are gated exactly like the SPA's own, so it can do nothing the user
  couldn't already do. (The bridge is where a future per-app restriction —
  e.g. read-only — would be enforced.)

**Trust level.** The sandbox protects the *browser* — it does **not** limit
what an app can do to your *data*. Because an app runs as the logged-in user,
its code can perform any create/update/delete/link the user can, and can invoke
any registered Lua action via `rela.action`. Treat an app folder with the
**same review rigor as a `scripts/` Lua action**: it is code, not content. Apps
live as files in `apps/`, versioned in git, and should go through the same
review as any other code.

## Best Practices

1. **Start with navigation** - Decide which entity types users will work with most, and create
   lists for those first. Add forms as needed. Consider adding a dashboard as the landing page
   for an at-a-glance overview.

2. **Create before edit** - Define a create form with sensible defaults and hidden fields (like
   `status: open`). Then define an edit form with transitions and all fields visible.

3. **Use `link: true`** on the primary column (usually `title` or `name`) so users can click
   through to entity details.

4. **Filter strategically** - Use static filters for focused views (e.g., "Open Tickets") and
   filter controls for exploratory views (e.g., "All Tickets").

5. **Group related lists** - Use navigation groups to organize related lists under collapsible
   headers. Keep 3-5 items per group for clarity.

6. **Style all enums** - Add color mappings for every custom type to make lists scannable.

7. **Views for key entities** - Create detail views for entities that aggregate related data.
   A risk detail view showing assets, controls, and incidents is more useful than viewing the
   risk entity alone.

## Audit log

Every edit performed through the data-entry app is recorded in
`.rela/audit/YYYY-MM-DD.jsonl` with `principal.tool: "data-entry"`.
The user is currently stamped as `"unknown"` — recording the server
process's OS user for every edit would be misleading. Per-request
user attribution (read from a header / cookie / session) lands in a
follow-up. See [audit-log.md](audit-log.md) for the full schema and
operator concerns.
