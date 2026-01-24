# Best Practices

This guide covers practical tips for maintaining a healthy information system
traceability graph over time.

## Removing Entities: Retire, Don't Delete

When a requirement, decision, or other entity is no longer needed, **don't
delete the file**. Instead, change its status to `retired` or `rejected`.

### Why?

1. **Preserve the audit trail** - In regulated environments (ISO 27001, SOC2),
   auditors may ask "what happened to REQ-042?" A retired entity with an
   explanation is better than a mysterious gap.

2. **Protect external references** - Entity IDs may be referenced in:
   - Git commit messages ("Implements REQ-042")
   - External documentation (Confluence, Word specs)
   - Issue trackers (JIRA tickets)
   - Meeting notes and emails

   Deleting the entity breaks these references silently.

3. **Maintain traceability history** - If REQ-042 was implemented by DEC-015,
   and you delete REQ-042, the decision now has an orphaned reference that's
   hard to explain.

### How to Retire an Entity

```bash
# Update the status
rela update REQ-042 --property "status=retired"

# Or for rejected requirements
rela update REQ-042 --property "status=rejected"
```

Then add an explanation in the entity's content:

```markdown
---
id: REQ-042
type: requirement
title: Support for legacy XML import
status: retired
---

## Retirement Note

Retired on 2024-03-15. This requirement was superseded by REQ-078 which uses
JSON import instead. The legacy system was decommissioned.
```

### Status Values for Removed Entities

See the table below for useful states:

| Status       | Use When                                           |
| ------------ | -------------------------------------------------- |
| `retired`    | The entity was valid but is no longer needed       |
| `rejected`   | The entity was considered but not accepted         |
| `superseded` | Replaced by another entity (reference it in notes) |

### Linking Superseded Entities

If one requirement replaces another, document the relationship:

```bash
# Create the new requirement
rela create requirement --title "JSON-based data import" --property "status=accepted"

# Retire the old one and link them
rela update REQ-042 --property "status=retired"
rela link REQ-078 supersedes REQ-042  # If your metamodel supports this relation
```

## Understanding ID Gaps

The `rela analyze gaps` command reports missing numbers in ID sequences. For
example, if you have REQ-001, REQ-002, and REQ-004, it will report REQ-003 as
missing.

### Gaps Are Normal

ID gaps are **not necessarily problems**. They naturally occur when:

- Requirements are retired or rejected
- Entities were created during exploration but not kept
- IDs were reserved but never used

### Don't Renumber

Resist the urge to "clean up" by renumbering entities. This causes:

- Broken external references
- Confusing Git history
- Lost audit trail
- Potential merge conflicts in team environments

### When to Investigate Gaps

Gaps deserve attention when:

- You can't explain why an ID is missing
- The gap appeared unexpectedly (possible accidental deletion)
- Auditors require documentation of all ID assignments

## Handling Orphan Entities

Orphans are entities with no relations—disconnected from the architecture graph.

### Legitimate Orphans

Some orphans are expected:

- **Work in progress**: New entities not yet linked
- **Standalone documentation**: Some decisions may be self-contained
- **Top-level requirements**: Parent requirements with no upstream dependencies

### Problematic Orphans

Investigate orphans that:

- Have been orphaned for a long time
- Should clearly be connected (a component with no solution)
- Were previously connected (check Git history)

### Resolving Orphans

```bash
# Find orphans
rela analyze orphans

# Option 1: Link them
rela link COMP-005 realizes SOL-003

# Option 2: Retire if no longer needed
rela update COMP-005 --property "status=retired"

# Option 3: Document why it's standalone (in the entity content)
```

## Maintaining Traceability Over Time

### Regular Health Checks

Run analysis regularly:

```bash
# Full analysis
rela analyze all

# In CI pipelines, fail on critical issues
rela analyze orphans && rela analyze cardinality
```

### Document as You Go

Don't wait until the end of a project:

- Create requirements when they're identified
- Document decisions when the context is fresh
- Link components as they're implemented

### Review in Pull Requests

Architecture changes should be reviewed like code:

```bash
# Show what changed
git diff entities/ relations/

# Verify constraints
rela analyze cardinality
```

### Cascade Updates

When something changes, follow the trace:

1. Requirement changes → Review affected decisions
2. Decision changes → Update implementing solutions
3. Solution changes → Verify component implementations

## Working with Statuses

### Status Lifecycle

A typical entity lifecycle:

```
draft → proposed → accepted → [retired|deprecated]
                           ↘ rejected
```

### Use Draft for Work in Progress

```bash
rela create requirement --title "TBD: Auth approach" --property "status=draft"
```

Draft entities:

- Signal incomplete work
- Can be excluded from cardinality analysis via validation rules
- Are visible reminders of open items

### Proposed for Review

```bash
rela update REQ-010 --property "status=proposed"
```

Use `proposed` when an entity is ready for team review but not yet approved.

### Accepted as Baseline

Only `accepted` entities should be considered part of the official architecture
baseline.

## Tips for Teams

### Consistent ID Patterns

Agree on ID patterns upfront:

```yaml
# In metamodel.yaml
entities:
  requirement:
    id_patterns: ["REQ-", "FR-", "NFR-"]
```

Use prefixes to categorize:

- `FR-` for functional requirements
- `NFR-` for non-functional requirements
- `SEC-` for security requirements

### Ownership

Consider adding an `owner` property to track responsibility:

```yaml
entities:
  requirement:
    properties:
      owner:
        type: string
        description: "Team or person responsible"
```

### Regular Cleanup Sessions

Schedule periodic reviews to:

- Retire obsolete entities
- Fix orphans and cardinality violations
- Update stale content
- Verify the graph reflects reality
