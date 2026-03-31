---
id: TKT-Y2JW
type: ticket
title: Add checklist validation for markdown content
kind: enhancement
priority: medium
effort: m
status: done
---

# Add Checklist Validation for Markdown Content

## Summary

Implement validation rules for markdown checklists (task lists) within entity bodies. This enables CI gates that require all tasks to be completed before status transitions.

## Use Cases

1. **CI gates**: Block status transitions when checklists are incomplete
2. **Workflow enforcement**: Ensure planning/implementation checklists are finished
3. **Quality assurance**: Verify all review items have been addressed

## Acceptance Criteria

- [ ] Can define `checklist.all-checked` validation rule
- [ ] Counts `- [ ]` and `- [x]` items correctly
- [ ] `allow-skipped` option handles strikethrough items with reasons
- [ ] Violations appear in `analyze_validations` output
- [ ] Works with `when:` conditions for status-based requirements
- [ ] Integration tests verify checklist parsing edge cases
