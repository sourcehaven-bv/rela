# TUI Guide

The rela Terminal User Interface (TUI) provides an interactive way to browse, create, and manage entities.

## Launching the TUI

```bash
rela tui
```

## Keyboard Shortcuts

Press `?` at any time to see the help screen.

### Global

| Key | Action |
|-----|--------|
| `q` | Quit / Go back |
| `Esc` | Go back |
| `?` | Show help |
| `/` | Search |
| `Ctrl+C` | Force quit |

### Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Select / Open |
| `Backspace` | Go back |
| `g` / `Home` | Go to top |
| `G` / `End` | Go to bottom |

## Screens

### Browser (Main Screen)

The browser is the starting point, showing all entity types and their counts.

```
┌─────────────────────────────────────────┐
│  rela - Browser                         │
├─────────────────────────────────────────┤
│                                         │
│  > Requirement (12)                     │
│    Decision (8)                         │
│    Solution (5)                         │
│    Component (15)                       │
│                                         │
├─────────────────────────────────────────┤
│  ↑/↓ navigate  Enter select  c create  │
└─────────────────────────────────────────┘
```

**Navigation:**
- Press `Enter` on a type to see its entities
- Press `Enter` on an entity to view its details

**Shortcuts:**
| Key | Action |
|-----|--------|
| `c` | Create new entity |
| `l` | Link from selected entity |
| `a` / `A` | Open Analysis screen |
| `m` / `M` | Open Metamodel screen |

### Entity Detail

Shows full entity information with relations.

```
┌─────────────────────────────────────────┐
│  rela - Entity Detail                   │
├─────────────────────────────────────────┤
│                                         │
│  REQ-001                                │
│  Status: accepted   Priority: high      │
│                                         │
│  System must support 1000 users         │
│                                         │
│  ─── Incoming Relations ───             │
│  > DEC-001 addresses this               │
│                                         │
│  ─── Outgoing Relations ───             │
│    (none)                               │
│                                         │
├─────────────────────────────────────────┤
│  Tab toggle  l link  g graph  e edit   │
└─────────────────────────────────────────┘
```

**Dual-mode navigation:**
- Default: Scroll through content
- Press `Tab` to switch to relation navigation mode
- In relation mode, press `Enter` to follow a relation

**Shortcuts:**
| Key | Action |
|-----|--------|
| `Tab` | Toggle scroll/relation mode |
| `Enter` | Follow selected relation |
| `l` | Create link from this entity |
| `g` | View relationship graph |
| `e` | Edit entity in `$EDITOR` |
| `E` | Edit selected relation in editor |

### Create Entity

Guided wizard for creating new entities.

**Step 1: Select Type**
```
┌─────────────────────────────────────────┐
│  rela - Create Entity                   │
├─────────────────────────────────────────┤
│                                         │
│  Select entity type:                    │
│                                         │
│  > Requirement                          │
│    Decision                             │
│    Solution                             │
│    Component                            │
│                                         │
└─────────────────────────────────────────┘
```

**Step 2: Enter Title**
```
┌─────────────────────────────────────────┐
│  rela - Create Requirement              │
├─────────────────────────────────────────┤
│                                         │
│  Title: System must handle scale█       │
│                                         │
│  Press Enter to create, Esc to cancel   │
│                                         │
└─────────────────────────────────────────┘
```

The entity ID is auto-generated based on the type's ID pattern.

### Search

Full-text search across all entities.

```
┌─────────────────────────────────────────┐
│  rela - Search                          │
├─────────────────────────────────────────┤
│                                         │
│  Search: scale█                         │
│                                         │
│  Results (3):                           │
│  > REQ-001 - System must handle scale   │
│    DEC-003 - Horizontal scaling design  │
│    SOL-001 - Auto-scaling solution      │
│                                         │
├─────────────────────────────────────────┤
│  Enter search  ↑/↓ results  Ctrl+U clear│
└─────────────────────────────────────────┘
```

Searches in:
- Entity IDs
- Titles
- Descriptions
- Free-form content

**Shortcuts:**
| Key | Action |
|-----|--------|
| `Enter` | Search (empty) or open result (with selection) |
| `Ctrl+U` | Clear search input |

### Link Wizard

Create relations between entities in two steps.

**Step 1: Select Relation Type**

Shows only relation types valid for the source entity.

```
┌─────────────────────────────────────────┐
│  rela - Link from DEC-001               │
├─────────────────────────────────────────┤
│                                         │
│  Select relation type:                  │
│                                         │
│  > addresses (→ Requirement)            │
│    dependsOn (→ Decision, Component)    │
│                                         │
└─────────────────────────────────────────┘
```

**Step 2: Select Target Entity**

Shows entities of compatible types. Type to filter.

```
┌─────────────────────────────────────────┐
│  rela - Link DEC-001 addresses...       │
├─────────────────────────────────────────┤
│                                         │
│  Filter: sec█                           │
│                                         │
│  > REQ-002 - Security requirements      │
│    REQ-005 - Secure data storage        │
│                                         │
└─────────────────────────────────────────┘
```

### Graph View

Visualize entity relationships as a tree.

```
┌─────────────────────────────────────────┐
│  rela - Graph: REQ-001 (depth: 2)       │
├─────────────────────────────────────────┤
│                                         │
│  REQ-001                                │
│  ├── addresses ← DEC-001                │
│  │   └── implements ← SOL-001           │
│  └── addresses ← DEC-002                │
│      └── implements ← SOL-002           │
│                                         │
├─────────────────────────────────────────┤
│  +/- depth  Enter focus  d detail       │
└─────────────────────────────────────────┘
```

**Shortcuts:**
| Key | Action |
|-----|--------|
| `+` / `=` | Increase depth (max 5) |
| `-` / `_` | Decrease depth |
| `Enter` | Focus on selected node (new graph view) |
| `d` | Open detail view of selected node |

Circular references are marked as "back" references to prevent infinite loops.

### Analysis

Run quality checks on your architecture.

```
┌─────────────────────────────────────────┐
│  rela - Analysis                        │
├─────────────────────────────────────────┤
│                                         │
│  Select analysis:                       │
│                                         │
│  > Orphans - Find isolated entities     │
│    Duplicates - Find similar titles     │
│    Gaps - Find missing IDs              │
│    Cardinality - Check constraints      │
│    All - Run all checks                 │
│                                         │
└─────────────────────────────────────────┘
```

After selecting a check, results are displayed with color-coded errors.

**Shortcuts:**
| Key | Action |
|-----|--------|
| `a` | Run all checks |
| `Enter` | Run selected check |
| `Esc` | Return to check selection |

### Metamodel

Browse your project's metamodel configuration.

```
┌─────────────────────────────────────────┐
│  rela - Metamodel                       │
├─────────────────────────────────────────┤
│  [Entity Types] [Relations]             │
│                                         │
│  requirement                            │
│    Label: Requirement                   │
│    Aliases: req                         │
│    ID Patterns: REQ-                    │
│    Properties:                          │
│      title (string)*                    │
│      description (string)               │
│      status (status)*                   │
│      priority (priority)                │
│                                         │
└─────────────────────────────────────────┘
```

**Shortcuts:**
| Key | Action |
|-----|--------|
| `Tab` / `e` / `r` | Switch between Entity Types and Relations tabs |

### Help

Press `?` from any screen to see keyboard shortcuts.

Scroll with `j`/`k` and press `?` or `Esc` to close.

## Tips

### External Editor

The TUI uses your `$EDITOR` environment variable (defaults to `vi`) when you press `e` to edit an entity. After saving and closing the editor, the entity reloads automatically.

```bash
# Use VS Code
export EDITOR="code --wait"

# Use nano
export EDITOR=nano
```

### Navigation Flow

The TUI maintains a navigation stack:
- `Enter` pushes a new screen
- `q`, `Esc`, or `Backspace` pops back to the previous screen
- The stack is unlimited, so you can drill deep into relationships

### Quick Entity Creation

From the browser, press `c` to create an entity. If you have an entity type selected, it pre-selects that type. If you have an entity selected, pressing `l` starts the link wizard from that entity.
