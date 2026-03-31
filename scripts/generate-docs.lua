-- generate-docs.lua
-- Generate documentation from rela entities, replacing mdcomp templates.
-- Usage: cd docs-project && rela script ../scripts/generate-docs.lua --output-dir=../docs
--
-- This script generates:
-- - *.md from guide entities
-- - tutorials/*.md from tutorial entities
-- - scenarios/*.md from scenario entities
--
-- For README.md, run with --output-dir=.. and pass "readme" as first argument:
--   rela script generate-docs.lua --output-dir=.. readme

-- Generate a single doc page (guide, tutorial, or scenario)
local function generate_doc_page(entity, output_path)
    local title = entity:prop("title", entity.id)
    local content = entity.content or ""

    local doc = "<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->\n\n"
    doc = doc .. "# " .. title .. "\n\n"
    doc = doc .. content

    rela.write_file(output_path, doc, {ensure_newline = true})
end

-- Generate doc pages for an entity type
-- path_prefix: prepended to output path (e.g., "tutorials/" or "" for root)
local function generate_entity_type(entity_type, path_prefix)
    local entities = rela.list_entities(entity_type)

    for _, entity in ipairs(entities) do
        local slug = entity:strip_prefix()
        local output_path = path_prefix .. slug .. ".md"
        generate_doc_page(entity, output_path)
    end

    return #entities
end

-- Generate README.md with dynamic entity lists
local function generate_readme(output_path)
    local guides = rela.sort_entities(rela.list_entities("guide"), "order")
    local tutorials = rela.sort_entities(rela.list_entities("tutorial"), "id")
    local scenarios = rela.sort_entities(rela.list_entities("scenario"), "id")

    local readme = [[<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

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

]]

    -- Add guide table
    readme = readme .. rela.md.entity_table(guides, {
        {"Document", function(e)
            return rela.md.link(e:prop("title", e.id), "docs/" .. e:strip_prefix() .. ".md")
        end},
        {"Description", "summary"}
    })

    readme = readme .. [[

### Tutorials

]]

    -- Add tutorial table
    readme = readme .. rela.md.entity_table(tutorials, {
        {"Tutorial", function(e)
            return rela.md.link(e:prop("title", e.id), "docs/tutorials/" .. e:strip_prefix() .. ".md")
        end},
        {"Description", "summary"}
    })

    readme = readme .. [[

### Scenarios

]]

    -- Add scenario table
    readme = readme .. rela.md.entity_table(scenarios, {
        {"Scenario", function(e)
            return rela.md.link(e:prop("title", e.id), "docs/scenarios/" .. e:strip_prefix() .. ".md")
        end},
        {"Description", "summary"}
    })

    readme = readme .. [[

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
]]

    rela.write_file(output_path, readme, {ensure_newline = true})
end

-- Main execution
-- Files are written relative to --output-dir (defaults to {project}/output)
local mode = rela.args[1] or "docs"

if mode == "readme" then
    -- Generate only README.md (use --output-dir=.. to write to project root)
    generate_readme("README.md")
    rela.output({ readme = true })
else
    -- Generate docs (use --output-dir=../docs)
    rela.output({
        guides = generate_entity_type("guide", ""),
        tutorials = generate_entity_type("tutorial", "tutorials/"),
        scenarios = generate_entity_type("scenario", "scenarios/")
    })
end
