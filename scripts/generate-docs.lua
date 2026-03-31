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

-- Helper: Ensure string ends with newline
local function ensure_newline(s)
    if s:sub(-1) ~= "\n" then
        return s .. "\n"
    end
    return s
end

-- Helper: Sort entities by a property
local function sort_by(entities, prop)
    local sorted = {}
    for _, e in ipairs(entities) do
        table.insert(sorted, e)
    end
    table.sort(sorted, function(a, b)
        local va = a.properties[prop] or ""
        local vb = b.properties[prop] or ""
        -- Handle numeric comparison for 'order'
        if prop == "order" then
            return tonumber(va) or 999 < tonumber(vb) or 999
        end
        return va < vb
    end)
    return sorted
end

-- Helper: Remove prefix from ID (e.g., "GUIDE-concepts" -> "concepts")
local function remove_prefix(id)
    return id:gsub("^[A-Z]+%-", "")
end

-- Generate a single doc page (guide, tutorial, or scenario)
local function generate_doc_page(entity, output_path)
    local title = entity.properties.title or entity.id
    local content = entity.content or ""

    local doc = "<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->\n\n"
    doc = doc .. "# " .. title .. "\n\n"
    doc = doc .. content

    rela.write_file(output_path, ensure_newline(doc))
end

-- Generate guide pages (output to --output-dir root)
local function generate_guides()
    local guides = rela.list_entities("guide")
    local count = 0

    for _, guide in ipairs(guides) do
        local slug = remove_prefix(guide.id)
        local output_path = slug .. ".md"
        generate_doc_page(guide, output_path)
        count = count + 1
    end

    return count
end

-- Generate tutorial pages (output to --output-dir/tutorials/)
local function generate_tutorials()
    local tutorials = rela.list_entities("tutorial")
    local count = 0

    for _, tutorial in ipairs(tutorials) do
        local slug = remove_prefix(tutorial.id)
        local output_path = "tutorials/" .. slug .. ".md"
        generate_doc_page(tutorial, output_path)
        count = count + 1
    end

    return count
end

-- Generate scenario pages (output to --output-dir/scenarios/)
local function generate_scenarios()
    local scenarios = rela.list_entities("scenario")
    local count = 0

    for _, scenario in ipairs(scenarios) do
        local slug = remove_prefix(scenario.id)
        local output_path = "scenarios/" .. slug .. ".md"
        generate_doc_page(scenario, output_path)
        count = count + 1
    end

    return count
end

-- Generate README.md with dynamic entity lists
local function generate_readme(output_path)
    local guides = sort_by(rela.list_entities("guide"), "order")
    local tutorials = sort_by(rela.list_entities("tutorial"), "id")
    local scenarios = sort_by(rela.list_entities("scenario"), "id")

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

| Document | Description |
| -------- | ----------- |
]]

    -- Add guide entries
    for _, guide in ipairs(guides) do
        local slug = remove_prefix(guide.id)
        local title = guide.properties.title or guide.id
        local summary = guide.properties.summary or ""
        readme = readme .. "| [" .. title .. "](docs/" .. slug .. ".md) | " .. summary .. " |\n"
    end

    readme = readme .. [[

### Tutorials

| Tutorial | Description |
| -------- | ----------- |
]]

    -- Add tutorial entries
    for _, tutorial in ipairs(tutorials) do
        local slug = remove_prefix(tutorial.id)
        local title = tutorial.properties.title or tutorial.id
        local summary = tutorial.properties.summary or ""
        readme = readme .. "| [" .. title .. "](docs/tutorials/" .. slug .. ".md) | " .. summary .. " |\n"
    end

    readme = readme .. [[

### Scenarios

| Scenario | Description |
| -------- | ----------- |
]]

    -- Add scenario entries
    for _, scenario in ipairs(scenarios) do
        local slug = remove_prefix(scenario.id)
        local title = scenario.properties.title or scenario.id
        local summary = scenario.properties.summary or ""
        readme = readme .. "| [" .. title .. "](docs/scenarios/" .. slug .. ".md) | " .. summary .. " |\n"
    end

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

    rela.write_file(output_path, ensure_newline(readme))
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
    local guide_count = generate_guides()
    local tutorial_count = generate_tutorials()
    local scenario_count = generate_scenarios()

    -- Output summary
    rela.output({
        guides = guide_count,
        tutorials = tutorial_count,
        scenarios = scenario_count
    })
end
