---
id: GUIDE-data-entry
type: guide
title: "Data Entry Web App"
status: published
order: 10
audience: intermediate
summary: "Config-driven web UI for entity management"
---

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
- **Commands** - User-defined scripts triggered from the UI with streamed results
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
    detail_view: ticket_report
    page_size: 25
```

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
| `detail_view`     | string | View name for the row detail action                         |
| `page_size`       | int    | Rows per page (default: 25)                                 |

### Column Options

| Field      | Type   | Description                                    |
| ---------- | ------ | ---------------------------------------------- |
| `property` | string | Property name to display                       |
| `label`    | string | Column header (defaults to property name)      |
| `sortable` | bool   | Column can be sorted by clicking the header    |
| `link`     | bool   | Cell value links to the entity's detail page   |

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
/v2/list/all_tasks?filter[status]=open
/v2/list/all_tasks?filter[due_date][lte]=$today
/v2/list/all_tasks?filter[tags][in][]=urgent&filter[tags][in][]=blocker
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

Views define read-only detail pages that traverse the entity graph to display related data.
They are the data-entry equivalent of the CLI's views.yaml concept, adapted for
rendering as HTML sections.

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
| `traverse` | list   | Graph traversal rules (same as views.yaml)     |
| `sections` | list   | Display sections                               |

### Entry

```yaml
entry:
  type: ticket   # Entity type of the entry entity
```

When a user opens a view, the entry entity is determined by the URL. For example,
clicking a list row that references `detail_view: ticket_report` opens the view for that
specific ticket entity.

### Traverse Rules

Traverse rules collect related entities into named collections. They work identically to
views.yaml traverse rules:

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
    detail_view: ticket_detail
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

## Relationship to views.yaml

The `views` section in `data-entry.yaml` uses the same traversal engine as the CLI's
views.yaml, but adapted for HTML rendering:

| Feature                | views.yaml (CLI)                     | data-entry.yaml views                |
| ---------------------- | ------------------------------------ | ------------------------------------ |
| Traversal rules        | Same `from`/`follow`/`collect_as`    | Same `from`/`follow`/`collect_as`    |
| Output                 | YAML/JSON data                       | HTML sections                        |
| Display control        | N/A (raw data)                       | `sections` with display modes        |
| Filters/derived        | `filters`, `derived`                 | Not yet supported                    |
| Relation exports       | `relation_exports`                   | Not yet supported                    |

If you already have a `views.yaml`, you can reuse the same traverse rules in your data-entry
views and add `sections` for HTML rendering.

## Analysis

The data entry app includes a built-in analysis page at `/analyze` that runs the same quality
checks as the CLI's `rela analyze all` command. It checks properties, cardinality constraints,
custom validations, orphans, duplicates, and ID gaps — displaying results grouped by category
with links to affected entities.

When a dashboard is configured, a validation summary card is automatically appended showing the
total error and warning counts with a link to the full analysis page.

No configuration is needed — the analysis page is always available in the sidebar.

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
