#!/bin/bash
# Migrate existing docs to rela entity files for the docs-project.
# This is a one-time migration script.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DOCS="$ROOT/docs"
ENTITIES="$ROOT/docs-project/entities"

# Helper: create entity file from an existing doc
# Usage: migrate_doc <source> <entity_file> <id> <type> <extra_frontmatter>
migrate_doc() {
    local src="$1"
    local dest="$2"
    local id="$3"
    local type="$4"
    local extra="$5"

    # Extract title from first # heading
    local title
    title=$(grep -m1 '^# ' "$src" | sed 's/^# //')

    # Get content after the first # heading line
    local content
    content=$(tail -n +2 "$src")

    # Strip leading blank lines from content
    content=$(echo "$content" | sed '/./,$!d')

    # Write entity file
    cat > "$dest" <<ENDOFFILE
---
id: $id
type: $type
title: "$title"
$extra
---

$content
ENDOFFILE

    echo "  Created $dest"
}

echo "Migrating guides..."
migrate_doc "$DOCS/getting-started.md" "$ENTITIES/guide/GUIDE-getting-started.md" \
    "GUIDE-getting-started" "guide" \
    "status: published
order: 1
audience: beginner
summary: \"Installation, first project, core workflow\""

migrate_doc "$DOCS/concepts.md" "$ENTITIES/guide/GUIDE-concepts.md" \
    "GUIDE-concepts" "guide" \
    "status: published
order: 2
audience: beginner
summary: \"Architecture traceability fundamentals\""

migrate_doc "$DOCS/cli-reference.md" "$ENTITIES/guide/GUIDE-cli-reference.md" \
    "GUIDE-cli-reference" "guide" \
    "status: published
order: 3
audience: intermediate
summary: \"Complete command reference\""

migrate_doc "$DOCS/metamodel.md" "$ENTITIES/guide/GUIDE-metamodel.md" \
    "GUIDE-metamodel" "guide" \
    "status: published
order: 4
audience: intermediate
summary: \"Configure entity types and relations\""

migrate_doc "$DOCS/views.md" "$ENTITIES/guide/GUIDE-views.md" \
    "GUIDE-views" "guide" \
    "status: published
order: 5
audience: intermediate
summary: \"Declarative views, CI integration\""

migrate_doc "$DOCS/tui-guide.md" "$ENTITIES/guide/GUIDE-tui.md" \
    "GUIDE-tui" "guide" \
    "status: published
order: 6
audience: beginner
summary: \"Interactive terminal interface\""

migrate_doc "$DOCS/export-guide.md" "$ENTITIES/guide/GUIDE-export.md" \
    "GUIDE-export" "guide" \
    "status: published
order: 7
audience: intermediate
summary: \"Export, import, and data integration\""

migrate_doc "$DOCS/best-practices.md" "$ENTITIES/guide/GUIDE-best-practices.md" \
    "GUIDE-best-practices" "guide" \
    "status: published
order: 8
audience: intermediate
summary: \"Maintenance tips and team workflows\""

migrate_doc "$DOCS/mcp-server.md" "$ENTITIES/guide/GUIDE-mcp-server.md" \
    "GUIDE-mcp-server" "guide" \
    "status: published
order: 9
audience: advanced
summary: \"AI assistant integration via MCP\""

echo ""
echo "Migrating tutorials..."
migrate_doc "$DOCS/tutorials/iso27001-isms-tutorial.md" "$ENTITIES/tutorial/TUT-iso27001-isms-tutorial.md" \
    "TUT-iso27001-isms-tutorial" "tutorial" \
    "status: published
audience: intermediate
summary: \"Build a complete Information Security Management System\""

migrate_doc "$DOCS/tutorials/project-management-tutorial.md" "$ENTITIES/tutorial/TUT-project-management-tutorial.md" \
    "TUT-project-management-tutorial" "tutorial" \
    "status: published
audience: intermediate
summary: \"Build a hybrid project management system\""

echo ""
echo "Migrating scenarios..."
migrate_doc "$DOCS/scenarios/iso27001-isms.md" "$ENTITIES/scenario/SCN-iso27001-isms.md" \
    "SCN-iso27001-isms" "scenario" \
    "status: published
domain: compliance
summary: \"ISO 27001 Information Security Management System\""

migrate_doc "$DOCS/scenarios/project-management.md" "$ENTITIES/scenario/SCN-project-management.md" \
    "SCN-project-management" "scenario" \
    "status: published
domain: project-management
summary: \"Hybrid project management documentation\""

migrate_doc "$DOCS/scenarios/devops-runbooks-infrastructure.md" "$ENTITIES/scenario/SCN-devops-runbooks.md" \
    "SCN-devops-runbooks" "scenario" \
    "status: published
domain: devops
summary: \"DevOps/SRE runbooks and infrastructure operations\""

echo ""
echo "Migration complete!"
echo "Run 'cd docs-project && ../bin/rela sync' to build the graph."
