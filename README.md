<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Rela

A database layer on top of markdown. Define entities, link them together, and query their
relationships—all stored as human-readable, version-controllable markdown files.

Rela lets you model any domain where traceability matters:

- **Architecture** — Link requirements to decisions to components
- **Compliance** — Connect controls to evidence and audit findings
- **Risk Management** — Trace risks through mitigations to controls
- **Product Development** — Map features to user stories to tasks
- **Knowledge Bases** — Relate concepts, documents, and references
- **Project Governance** — Track goals through milestones to deliverables

Define your own entity types and relationships in a simple YAML metamodel. Rela handles the rest:
ID generation, bidirectional linking, orphan detection, coverage analysis, and graph export.

## Quick Start

```bash
# Initialize a project
rela init

# Create entities
rela create requirement --title "System must support 1000 users"
rela create decision --title "Use PostgreSQL for persistence"

# Link them together
rela link DEC-001 addresses REQ-001

# View the relationship
rela show REQ-001

# Launch the interactive TUI
rela tui

# Work with a project in a different directory
rela -p /path/to/project list
export RELA_PROJECT=/path/to/project && rela list
```

## Features

- **Entity Management** - Create, update, delete entities
- **Relationship Tracing** - Link entities and trace dependencies
- **Quality Analysis** - Find orphans, check coverage, detect gaps
- **Graph Export** - Export to Graphviz DOT format
- **Interactive TUI** - Full-featured terminal interface
- **MCP Server** - Expose rela to AI assistants via Model Context Protocol
- **Markdown Storage** - Human-readable, version-controllable files

## Installation

```bash
go install github.com/Sourcehaven-BV/rela/cmd/rela@latest
```

Or build from source:

```bash
git clone https://github.com/Sourcehaven-BV/rela.git
cd rela
go build -o rela ./cmd/rela
```

## Documentation

| Document | Description |
| -------- | -------- |

### Tutorials

| Tutorial | Description |
| -------- | -------- |

### Scenarios

| Scenario | Description |
| -------- | -------- |

## Project Structure

After running `rela init`:

```text
your-project/
├── metamodel.yaml       # Entity types and relations config
├── entities/            # Markdown entity files (by type)
│   ├── requirements/
│   ├── decisions/
│   └── ...
├── relations/           # Markdown relation files
├── templates/           # Optional: templates for new entities/relations
│   ├── entities/        # One template per entity type
│   └── relations/       # One template per relation type
└── .rela/               # Cache (gitignored)
```

## Core Traceability Chain

```text
Requirement ──addresses──> Decision ──implements──> Solution ──realizes──> Component
```

Use `rela analyze coverage` to check for gaps in this chain.

## License

AGPL-3.0 - See [LICENSE](LICENSE) for details.
