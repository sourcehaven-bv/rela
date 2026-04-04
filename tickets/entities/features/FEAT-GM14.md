---
id: FEAT-GM14
type: feature
title: Checklist validation for markdown content
description: Add validation rules for markdown checklists (task lists) within entity bodies, enabling CI gates that require all tasks to be completed
status: implemented
---

# Checklist Validation for Markdown Content

Add validation rules for markdown checklists (task lists) within entity bodies. This enables enforcing completeness of checklists, which is useful for:

- **CI gates**: Ensuring all tasks in a planning/implementation checklist are completed before status transitions
- **Quality gates**: Verifying review checklists are finished before marking tickets done
- **Compliance**: Ensuring all required steps in a process have been acknowledged

## Design

### Syntax

Extend validation rules with checklist validation:

```yaml
validations:
  - name: done-tickets-need-complete-checklist
    entity_type: ticket
    when:
      - "status=done"
    content:
      checklist:
        all-checked: true
    severity: error

  - name: planning-checklists-complete
    entity_type: planning-checklist
    when:
      - "status=done"
    content:
      checklist:
        all-checked: true
        allow-skipped: true  # Allow strikethrough items with reason
    severity: error
```

### Checklist Options

| Option | Description |
|--------|-------------|
| `all-checked: true` | All `- [ ]` must be `- [x]` |
| `allow-skipped: true` | Strikethrough items (~~item~~) count as complete |
| `min-checked: N` | At least N items must be checked |
| `min-percentage: N` | At least N% of items must be checked |

### Skipped Items

Items can be marked as skipped using strikethrough with a reason:

```markdown
- [x] ~~API docs updated~~ (N/A: no API changes)
```

When `allow-skipped: true`, these count as complete. When false (default), only `- [x]` items count.

## Implementation

1. Add `ChecklistRule` type to `internal/metamodel/types.go`
2. Extend `ContentRule` with `Checklist *ChecklistRule` field
3. Add checklist parsing in `internal/markdown/` to extract task items
4. Add checklist validation logic in `internal/dataentry/analyze.go`
5. Count checked vs unchecked items, handle strikethrough detection
6. Report violations through existing `analyze_validations` flow

## Acceptance Criteria

- [ ] Can define `checklist.all-checked` in validation rules
- [ ] Counts `- [ ]` and `- [x]` items correctly
- [ ] `allow-skipped` handles strikethrough items
- [ ] `min-checked` and `min-percentage` work correctly
- [ ] Violations appear in `analyze_validations` output
- [ ] Works with `when:` conditions
