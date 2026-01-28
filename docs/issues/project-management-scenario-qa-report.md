# QA Report: Project Management Scenario Support

**Date:** 2026-01-24 **Scenario:** `docs/scenarios/project-management.md` **Rela
Version:** dev

## Summary

This report documents issues found while testing Rela's ability to support the
hybrid project management scenario. The scenario defines a comprehensive
metamodel with 13 entity types and 21 relation types across strategic, planning,
execution, decision, risk, stakeholder, and knowledge layers.

## Test Environment

- Built Rela from source (`go build -o rela ./cmd/rela`)
- Created test project at `/tmp/rela-pm-test`
- Used the metamodel defined in `docs/scenarios/project-management.md`
- Tested both CLI and TUI (via tmux)

## Issues Found

### Critical Issues

#### 1. CLI: Cannot create entities with non-"title" required properties

**Severity:** Critical **Component:** CLI (`create` command)

The `rela create` command only supports `--title` (`-t`) flag for the primary
field. Entity types that use a different required property name (like `name` for
stakeholder) cannot be created via CLI.

**Reproduction:**

```bash
# This fails because stakeholder requires "name", not "title"
rela create stakeholder -t "John Smith"
# Error: validation errors: missing required property: name

# There is no --name flag available
rela create stakeholder --name "John Smith"
# Error: unknown flag: --name
```

**Impact:** Entity types that don't use `title` as their required property are
completely inaccessible via CLI for creation.

**Affected entity types from PM scenario:**

- `stakeholder` (requires `name`)

**Suggested fix:** Either:

1. Add dynamic flag support based on metamodel properties, or
2. Add a generic `--property key=value` flag, or
3. Require `title` as the standard required property in the metamodel schema

---

#### 2. TUI: Entity creation only prompts for "Title" regardless of metamodel

**Severity:** Critical **Component:** TUI (Create wizard)

The TUI create wizard hardcodes "Title:" as the only input field, ignoring the
actual required properties defined in the metamodel. This creates entities that
fail validation or have incorrect data.

**Reproduction:**

1. Open TUI: `rela tui`
2. Press `c` to create
3. Select "Stakeholder"
4. Note: Only "Title:" is shown, not "Name:"
5. Enter a name and press Enter
6. Entity is created with empty `title` field, missing required `name`

**Result:**

```yaml
# Created STK-001.md
id: STK-001
status: draft
title: " " # Should be "name: John Smith"
type: stakeholder
```

**Impact:** Entities created via TUI may have invalid data and missing required
fields.

---

#### 3. TUI: Create wizard captures input incorrectly

**Severity:** Critical **Component:** TUI (Create wizard)

When creating entities via TUI, the title/name input is not captured correctly.
The created entity has whitespace instead of the entered text.

**Reproduction:**

1. Open TUI: `rela tui`
2. Press `c` to create
3. Select "Goal"
4. Type "Reduce time-to-market by 20%"
5. Press Enter
6. Check created entity

**Result:**

```yaml
# GOAL-002.md
id: GOAL-002
status: draft
title: "   " # Should be "Reduce time-to-market by 20%"
type: goal
```

**Impact:** Entities created via TUI are effectively blank/unusable.

---

### Major Issues

#### 4. Analysis: Coverage check uses hardcoded entity types

**Severity:** Major **Component:** CLI (`analyze coverage`)

The coverage analysis hardcodes the default metamodel types (`requirement`,
`decision`, `solution`) rather than using the entity types defined in the custom
metamodel.

**Reproduction:**

```bash
rela analyze coverage
# Output: "⚠ Decisions without solutions (1):"
```

For a project management metamodel, this should check:

- Goals without contributing features/epics
- Features without implementing tasks
- Etc.

**Impact:** Coverage analysis is not useful for custom metamodels.

**Suggested fix:** Make coverage analysis configurable via metamodel, defining
which chains to check (e.g., `goal -> epic -> feature -> task`).

---

#### 5. Trace: `trace from` doesn't traverse inverse relationships

**Severity:** Major **Component:** CLI (`trace from`)

The `trace from` command only shows direct outgoing relationships, not items
that link TO the entity via inverse relations.

**Reproduction:**

```bash
# Setup: GOAL-001 <- contributesTo <- EPIC-001 <- partOfEpic <- FEAT-001

rela trace from GOAL-001
# Output: Only shows "GOAL-001 Increase customer retention by 15%"
# Expected: Should also show EPIC-001 and FEAT-001 (things that contribute to the goal)

rela trace to TASK-001
# Works correctly: shows FEAT-001, RISK-001, DEC-001
```

**Impact:** Cannot trace the full dependency tree from a goal down to tasks.

---

#### 6. Trace: `trace path` fails to find existing paths

**Severity:** Major **Component:** CLI (`trace path`)

The path finding algorithm doesn't find paths that exist.

**Reproduction:**

```bash
# Path exists: GOAL-001 <- EPIC-001 <- FEAT-001 <- TASK-001
rela trace path GOAL-001 TASK-001
# Output: "No path found between GOAL-001 and TASK-001"
```

**Impact:** Path tracing between entities is unreliable.

---

### Minor Issues

#### 7. TUI: Unexpected exits when pressing certain keys

**Severity:** Minor **Component:** TUI

The TUI sometimes exits unexpectedly when pressing keys in certain contexts,
particularly when pressing `q` multiple times or transitioning between screens
quickly.

**Impact:** Requires restarting the TUI, which is mildly disruptive.

---

#### 8. TUI: Link wizard doesn't show confirmation

**Severity:** Minor **Component:** TUI (Link wizard)

After creating a link via the TUI wizard, there's no confirmation message. The
user is returned to the entity detail view, and they must manually verify the
link was created.

**Impact:** User experience - no feedback on success.

---

#### 9. Entity display shows empty title for some entities

**Severity:** Minor **Component:** TUI

In the link wizard target selection, entities with empty titles show as just
their ID with no title.

**Example:** `GOAL-002     (goal)` instead of `GOAL-002 <title> (goal)`

**Impact:** Confusing when selecting link targets.

---

## Features Working Well

### CLI

- Entity creation with standard `title` property
- Entity listing and filtering
- Entity show with relations
- Relationship creation (`link` command)
- Relationship removal (`unlink` command)
- Export to JSON/CSV/YAML
- Graph export to DOT format
- Update command
- Delete command (not tested with cascade)
- Orphan analysis
- Duplicate detection
- ID gap analysis
- Cardinality validation

### TUI

- Entity type browsing
- Entity list navigation
- Entity detail view with incoming/outgoing relations
- Relationship graph visualization
- Search functionality
- Link wizard (selecting relation type and target)
- Metamodel browser
- Help screens
- Analysis view (orphans, duplicates, gaps, cardinality)

### General

- Custom metamodel loading and validation
- 13 entity types from PM scenario correctly recognized
- 21 relation types correctly constrained
- Relation inverses working
- ID auto-generation with correct prefixes
- Status defaults applied correctly
- Property types validated

## Recommendations

### High Priority

1. Fix TUI create wizard to capture input correctly
2. Add dynamic property support to CLI create command
3. Make TUI create wizard read required properties from metamodel
4. Fix `trace from` to follow inverse relations
5. Fix `trace path` algorithm

### Medium Priority

1. Make coverage analysis configurable for custom metamodels
2. Add create confirmation in TUI
3. Add property editing support in TUI

### Low Priority

1. Improve TUI stability when rapidly pressing keys
2. Add visual feedback for successful operations in TUI

## Test Data Created

The following entities and relations were created during testing:

**Entities:**

- GOAL-001: Increase customer retention by 15%
- GOAL-002: (empty title - TUI bug)
- EPIC-001: Self-service account management
- FEAT-001: Password reset flow
- TASK-001: Implement email verification
- RISK-001: Third-party API deprecation
- ISS-001: Stripe API v2 sunset notice
- DEC-001: Use GraphQL over REST
- MTG-001: Architecture review 2024-03-15
- MS-001: Phase 1 Release
- BUG-001: Login timeout bug
- RETRO-001: Sprint 23 retro
- IMP-001: Add definition of done checklist
- STK-001: (empty - validation bug)

**Relations:**

- EPIC-001 contributesTo GOAL-001
- FEAT-001 partOfEpic EPIC-001
- FEAT-001 implementedBy TASK-001
- FEAT-001 contributesTo GOAL-002
- RISK-001 threatens FEAT-001
- RISK-001 becameIssue ISS-001
- DEC-001 affects FEAT-001
- DEC-001 decidedIn MTG-001
- RETRO-001 produces IMP-001

## Conclusion

Rela provides a solid foundation for the project management scenario with
excellent support for custom metamodels, flexible relationships, and
comprehensive analysis tools. However, critical bugs in entity creation (both
CLI and TUI) and major limitations in trace functionality currently prevent full
adoption of the scenario. Once these issues are addressed, Rela should be
well-suited for hybrid project management documentation.
