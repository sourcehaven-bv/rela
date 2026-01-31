# Data Entry Configuration

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
- **Navigation** - Sidebar menu entries that link to lists or the dashboard

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

navigation:                # Sidebar menu
  - label: "Dashboard"
    dashboard: true
  - label: "Tasks"
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
  require_pr: [main, production]
```

| Field        | Description                                                           |
| ------------ | --------------------------------------------------------------------- |
| `require_pr` | List of branch names where direct push is blocked (protected branches) |

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
| `widget`       | string | `"select"`, `"multi-select"`, or `"search"`                   |
| `allow_create` | bool   | Show an inline "create new" button                             |
| `create_form`  | string | Form name to use for inline creation                           |
| `properties`   | list   | Editable properties on the relation itself                     |

**Relation widget types:**

| Widget         | Description                                                  |
| -------------- | ------------------------------------------------------------ |
| `select`       | Dropdown listing all entities of the target type (pick one)  |
| `multi-select` | Tag-style picker for selecting multiple entities             |
| `search`       | Type-ahead search field for large entity sets                |

**Inline creation:** When `allow_create: true` and `create_form` is set, a button appears next to
the relation picker. Clicking it opens a modal with the referenced form, and the newly created
entity is automatically linked.

### Relation Properties

Relations can have their own editable properties:

```yaml
relations:
  - relation: blocks
    direction: outgoing
    target_type: ticket
    label: "Blocks"
    widget: search
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
| `operator` | string | `"="`, `"!="`, `"<"`, `"<="`, `">"`, `">="` |
| `value`    | string | Value to compare against                 |

**Special values:**

- `$USER` - Replaced with the current system username at runtime

```yaml
filters:
  - property: assignee
    operator: "="
    value: "$USER"
```

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

### Sort Configuration

```yaml
sort:
  property: priority
  direction: asc    # "asc" or "desc"
```

## Views

Views define read-only detail pages that traverse the entity graph to display related data.
They are the data-entry equivalent of the CLI's [views.yaml](views.md) concept, adapted for
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
[views.yaml traverse rules](views.md):

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
```

| Field             | Type   | Description                                        |
| ----------------- | ------ | -------------------------------------------------- |
| `from`            | string | Source: `"entry"` or a collection name              |
| `follow`          | string | Outgoing relation type to follow                   |
| `follow_incoming` | string | Incoming relation type to follow (reverse)         |
| `collect_as`      | string | Name for the collected entities                    |
| `recursive`       | bool   | Follow the relation transitively                   |
| `max_depth`       | int    | Maximum recursion depth                            |

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
| free text                | `authentication`                  | Substring match across all fields|
| `"quoted phrase"`        | `"REST API"`                      | Exact phrase match               |

Multiple terms are combined with AND logic. For example,
`type:ticket status:open prop:priority=critical` matches tickets that are both open and critical.

Every card includes a link icon (↗) that opens the same query on the search page for further
exploration.

## Navigation

The navigation section defines the sidebar menu. Each entry links to a named list or the dashboard:

```yaml
navigation:
  - label: "Dashboard"
    dashboard: true
  - label: "Open Tickets"
    list: open_tickets
  - label: "All Tickets"
    list: all_tickets
  - label: "Categories"
    list: categories
```

| Field       | Type   | Description                                              |
| ----------- | ------ | -------------------------------------------------------- |
| `label`     | string | Menu item text                                           |
| `list`      | string | List name to navigate to (mutually exclusive with `dashboard`) |
| `dashboard` | bool   | Link to the dashboard page (mutually exclusive with `list`)    |

The first entry is the default landing page. If the first entry is a dashboard, the root URL
(`/`) shows the dashboard. Order matters; items appear in the sidebar in the order listed.

List entries show an entity count badge next to the label (based on the list's filters). Dashboard
entries do not show a count.

## Complete Example

A ticket management system with forms, lists, views, and navigation:

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

navigation:
  - label: "Dashboard"
    dashboard: true
  - label: "My Tickets"
    list: my_tickets
  - label: "Open Tickets"
    list: open_tickets
  - label: "All Tickets"
    list: all_tickets
```

## Relationship to views.yaml

The `views` section in `data-entry.yaml` uses the same traversal engine as the CLI's
[views.yaml](views.md), but adapted for HTML rendering:

| Feature                | views.yaml (CLI)                     | data-entry.yaml views                |
| ---------------------- | ------------------------------------ | ------------------------------------ |
| Traversal rules        | Same `from`/`follow`/`collect_as`    | Same `from`/`follow`/`collect_as`    |
| Output                 | YAML/JSON data                       | HTML sections                        |
| Display control        | N/A (raw data)                       | `sections` with display modes        |
| Filters/derived        | `filters`, `derived`                 | Not yet supported                    |
| Relation exports       | `relation_exports`                   | Not yet supported                    |

If you already have a `views.yaml`, you can reuse the same traverse rules in your data-entry
views and add `sections` for HTML rendering.

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

5. **Keep navigation focused** - 8-15 items is a comfortable sidebar. Group related lists
   logically (e.g., ISMS items first, then Sales items).

6. **Style all enums** - Add color mappings for every custom type to make lists scannable.

7. **Views for key entities** - Create detail views for entities that aggregate related data.
   A risk detail view showing assets, controls, and incidents is more useful than viewing the
   risk entity alone.

## See Also

- [Views](views.md) - Declarative views for the CLI
- [Metamodel](metamodel.md) - Entity types and relations that forms/lists reference
- [Getting Started](getting-started.md) - Project initialization
