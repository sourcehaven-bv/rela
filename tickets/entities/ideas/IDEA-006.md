---
category: feature
description: Trigger actions when properties change, like Notion automations. E.g., when bug status becomes done, validate 5-whys fields are filled and create a relation to the current QA report.
effort: medium
id: IDEA-006
inspiration: Notion database automations
status: promoted
title: Database Automations
type: idea
value: valuable
---

# Design Exploration

## Core Location

The automation engine would live in `internal/automation/`:

```
internal/automation/
├── engine.go      # Core trigger/action loop
├── trigger.go     # Trigger detection (diff old vs new state)
├── actions.go     # Action executors
└── template.go    # Variable interpolation
```

Hook points:
- `markdown.WriteEntity()` - fires after entity save
- `graph.AddRelation()` / `RemoveRelation()` - relation changes
- File watcher could also trigger on external edits

## Trigger Types

| Trigger | Example |
|---------|---------|
| Property change | `status: backlog → in-progress` |
| Property equals | `priority = critical` |
| Relation created | `bug --fixes→ feature` |
| Entity created | New `ticket` entity |
| Time-based | `due_date` is past today |

## Action Types

| Action | Example |
|--------|---------|
| **Validate** | Block save if 5-whys not filled when bug is done |
| **Set property** | Auto-set `started_at` when status → in-progress |
| **Create relation** | Auto-link bug to current sprint |
| **Create entity** | Spawn doc-task when feature → implemented |
| **Webhook** | POST to Slack when priority = critical |
| **Run command** | Execute shell script for notifications |

## Validation - Blocking vs Advisory

| Mode | Behavior | UX |
|------|----------|-----|
| **block** | Reject save, return error | Hard gate, user must fix |
| **warn** | Save succeeds, show warning | Advisory, user can ignore |
| **fix** | Auto-fix if possible, else block | Smart defaults |

Context-specific behavior:
- **CLI**: `--strict` flag to enable blocking
- **Server**: Show warning banner, allow "Save Anyway" button
- **Git hook**: Block on commit (pre-commit validation)

Validation rules could have `severity: error | warning | info`.

## User Identity

Rela currently has no user concept. Options:

| Approach | How it works | Complexity |
|----------|--------------|------------|
| **Git author** | Read from `git config user.name/email` | Low |
| **Env var** | `RELA_USER` environment variable | Low |
| **Config file** | `.rela/user.yaml` with name/email | Low |
| **OS user** | `os.Getenv("USER")` | Trivial |

Git author seems natural since rela is file-based and often used with git.

## Template Variables

```yaml
{{now}}           # Current timestamp (ISO 8601)
{{today}}         # Current date
{{user.name}}     # From git config or env
{{user.email}}    # From git config
{{entity.id}}     # Current entity ID
{{entity.type}}   # Current entity type
{{old.status}}    # Previous value of property
{{new.status}}    # New value of property
```

## Proposed Syntax

```yaml
# In metamodel.yaml
automations:
  track-started:
    on:
      entity: [ticket, bug]
      property: status
      becomes: in-progress
    do:
      - set: started_at
        value: "{{today}}"
      - set: started_by
        value: "{{user.name}}"

  require-analysis:
    on:
      entity: bug
      property: status
      becomes: done
    validate:
      - check: why1 != ""
        severity: error
        message: "Root cause (why1) is required"
      - check: prevention != ""
        severity: warning
        message: "Consider documenting prevention steps"

  critical-bug-notify:
    on:
      entity: bug
      property: priority
      becomes: critical
    do:
      - webhook:
          url: "${SLACK_WEBHOOK}"
          body: "🚨 Critical bug: {{entity.title}}"
```
