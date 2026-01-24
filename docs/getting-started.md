# Getting Started

This guide walks you through installing rela and creating your first project.

## Installation

### Using Go

```bash
go install github.com/Sourcehaven-BV/rela/cmd/rela@latest
```

### Building from Source

```bash
git clone https://github.com/Sourcehaven-BV/rela.git
cd rela
go build -o rela ./cmd/rela

# Optionally install to your PATH
mv rela /usr/local/bin/
```

### Shell Completion

Enable tab completion for your shell:

```bash
# Bash
rela completion bash > /etc/bash_completion.d/rela

# Zsh
rela completion zsh > "${fpath[1]}/_rela"

# Fish
rela completion fish > ~/.config/fish/completions/rela.fish

# PowerShell
rela completion powershell > rela.ps1
```

## Initialize a Project

Create a new rela project in your current directory:

```bash
mkdir my-project
cd my-project
rela init
```

This creates:
- `metamodel.yaml` - Defines your entity types and relations
- `entities/` - Where entity markdown files are stored
- `relations/` - Where relation markdown files are stored
- `.rela/` - Cache directory (automatically added to `.gitignore`)

Optionally, you can generate templates to customize default values for new entities:

```bash
rela template init
```

This creates `templates/entities/` and `templates/relations/` with template files for each type.

## Your First Entities

Create a requirement:

```bash
rela create requirement --title "System must handle 1000 concurrent users"
# Creates REQ-001
```

Create a decision that addresses this requirement:

```bash
rela create decision --title "Use horizontal scaling with load balancer"
# Creates DEC-001
```

Link them together:

```bash
rela link DEC-001 addresses REQ-001
```

## Viewing Your Work

List all entities:

```bash
rela list
```

List by type:

```bash
rela list requirements
rela list decisions
```

View a specific entity with its relations:

```bash
rela show REQ-001
```

## Using the TUI

For a more interactive experience, launch the terminal UI:

```bash
rela tui
```

Navigate with arrow keys or `j`/`k`, press `?` for help.

## Next Steps

- [CLI Reference](cli-reference.md) - All commands and options
- [TUI Guide](tui-guide.md) - Master the interactive interface
- [Metamodel](metamodel.md) - Customize entity types and relations
- [Concepts](concepts.md) - Understand architecture traceability

### Tutorials

- [ISO 27001 ISMS Tutorial](tutorials/iso27001-isms-tutorial.md) - Build a complete Information Security Management System

## Common Workflows

### Building a Traceability Chain

```bash
# Start with requirements
rela create requirement --title "Users must authenticate before accessing data"

# Make design decisions
rela create decision --title "Implement OAuth 2.0 with JWT tokens"
rela link DEC-001 addresses REQ-001

# Define solutions
rela create solution --title "Auth service with Redis token storage"
rela link SOL-001 implements DEC-001

# Track components
rela create component --title "auth-service container"
rela link COMP-001 realizes SOL-001
```

### Quality Checks

Find orphaned entities with no connections:

```bash
rela analyze orphans
```

Run all quality checks:

```bash
rela analyze all
```

### Bulk Import

Import multiple entities at once from JSON, YAML, or CSV:

```bash
# Import from JSON
rela import entities.json

# Import from YAML
rela import data.yaml

# Import from CSV (for spreadsheet data)
rela import entities.csv

# Validate before importing
rela import --dry-run data.json
```

See [Export Guide](export-guide.md#importing-data) for details on import formats.

### Generating Documentation

Export a visual graph:

```bash
# Generate DOT file
rela graph -o architecture.dot

# Render to PNG (requires Graphviz)
rela graph -o architecture.png -f png
```

### Tracing Dependencies

See what depends on an entity:

```bash
rela trace from REQ-001
```

See what an entity depends on:

```bash
rela trace to COMP-001
```

Find the path between two entities:

```bash
rela trace path REQ-001 COMP-001
```
