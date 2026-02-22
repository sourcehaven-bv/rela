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

# Work with a project in a different directory
rela -p /path/to/project list
export RELA_PROJECT=/path/to/project && rela list
```

## Features

- **Entity Management** - Create, update, delete entities
- **Relationship Tracing** - Link entities and trace dependencies
- **Quality Analysis** - Find orphans, check coverage, detect gaps
- **Graph Export** - Export to Graphviz DOT format
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
| -------- | ----------- |
| [Getting Started](docs/getting-started.md) | Installation, first project, core workflow |
| [Concepts](docs/concepts.md) | Architecture traceability fundamentals |
| [CLI Reference](docs/cli-reference.md) | Complete command reference |
| [Metamodel Reference](docs/metamodel.md) | Configure entity types and relations |
| [Views - Declarative Context Generation](docs/views.md) | Declarative views, CI integration |
| [Export Guide](docs/export.md) | Export, import, and data integration |
| [Best Practices](docs/best-practices.md) | Maintenance tips and team workflows |
| [MCP Server](docs/mcp-server.md) | AI assistant integration via MCP |
| [Data Entry Web App](docs/data-entry.md) | Config-driven web UI for entity management |

### Tutorials

| Tutorial | Description |
| -------- | ----------- |
| [Tutorial: Building an ISO 27001 ISMS with Rela](docs/tutorials/iso27001-isms-tutorial.md) | Build a complete Information Security Management System |
| [Tutorial: Hybrid Project Management with Rela](docs/tutorials/project-management-tutorial.md) | Build a hybrid project management system |

### Scenarios

| Scenario | Description |
| -------- | ----------- |
| [Scenario: DevOps/SRE Runbooks & Infrastructure Operations](docs/scenarios/devops-runbooks.md) | DevOps/SRE runbooks and infrastructure operations |
| [Scenario: ISO 27001 Information Security Management System](docs/scenarios/iso27001-isms.md) | ISO 27001 Information Security Management System |
| [Scenario: Hybrid Project Management](docs/scenarios/project-management.md) | Hybrid project management documentation |

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
