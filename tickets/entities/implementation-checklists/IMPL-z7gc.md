---
id: IMPL-z7gc
status: done
title: 'Implementation: Add metamodel cleanup/trim command'
type: implementation-checklist
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. **CLI `rela analyze schema`** - Tested on tickets project, shows unused entity types, relation types, and custom types with their references
2. **`--threshold` flag** - Works correctly to include low-usage types  
3. **`--cleanup --dry-run`** - Shows planned changes without modifying files
4. **JSON output** - `-o json` produces valid JSON with status, message, count, and details
5. **Reference tracking** - Correctly identifies references in metamodel.yaml (relations, validations, automations) and data-entry.yaml (forms, lists, views)
6. **Safe removal logic** - Only types without blocking references (forms, lists, views, validations, automations) are marked for cleanup

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
