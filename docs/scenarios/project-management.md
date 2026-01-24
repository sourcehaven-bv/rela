# Scenario: Hybrid Project Management

## Background

Project management documentation sprawls across multiple tools: requirements in Jira, decisions in Confluence, risks in spreadsheets, stakeholder info in email threads, and meeting notes in Google Docs. Each tool optimizes for its narrow purpose but none provides the connections between artifacts that project managers desperately need.

When a stakeholder asks "why did we decide to delay feature X?", the PM must hunt through meeting notes, Jira tickets, and Slack history. When priorities shift, there's no systematic way to assess impact on dependent work. When projects end, institutional knowledge evaporates.

Modern projects rarely follow pure methodologies. Teams blend agile ceremonies with traditional planning, use Kanban for operations while running sprints for features, and adapt their process to project phase and team maturity. Documentation tools should support this pragmatic reality, not force artificial methodology purity.

## Context

A pragmatic project management documentation system needs:

1. **Strategic Layer** - Goals, OKRs, success criteria that drive the project
2. **Planning Layer** - Epics, features, milestones that structure the work
3. **Execution Layer** - Tasks, stories, bugs that represent actual work
4. **Decision Layer** - Choices made, alternatives considered, rationale captured
5. **Risk Layer** - Identified risks, mitigations, issues that materialize
6. **Stakeholder Layer** - Who cares, what they need, how to communicate
7. **Knowledge Layer** - Learnings, retrospective outcomes, process improvements

The critical relationships:
- Goals decompose into features
- Features are blocked by risks
- Decisions affect features
- Stakeholders own or care about goals
- Risks become issues
- Issues block tasks
- Retrospectives produce improvements

## Goals

### Primary Goals

1. **Traceability to Purpose** - Every task traces back to a goal; no orphan work
2. **Decision Memory** - Never re-debate a decision; the rationale is documented
3. **Risk Visibility** - Risks linked to what they threaten, mitigations tracked
4. **Stakeholder Clarity** - Know who cares about what, communication history preserved

### Secondary Goals

5. **Status Reporting** - Generate status from actual work state, not manual updates
6. **Impact Assessment** - When goals change, see what features/tasks are affected
7. **Retrospective Follow-through** - Improvement actions tracked to completion
8. **Knowledge Preservation** - Project learnings survive team transitions

## Proposed Metamodel

```yaml
version: "1.0"
namespace: "https://example.org/ontology/project#"

types:
  status:
    values: [draft, active, on_hold, completed, cancelled]
    default: draft

  priority:
    values: [critical, high, medium, low]

  work_status:
    values: [backlog, ready, in_progress, review, done, blocked]
    default: backlog

  risk_status:
    values: [identified, analyzing, mitigating, accepted, resolved, occurred]
    default: identified

  likelihood:
    values: [almost_certain, likely, possible, unlikely, rare]

  impact:
    values: [severe, major, moderate, minor, negligible]

  decision_status:
    values: [proposed, evaluating, decided, superseded, reversed]
    default: proposed

entities:
  # Strategic Layer
  goal:
    label: Goal
    aliases: [obj, okr]
    id_patterns: ["GOAL-", "OKR-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      success_criteria:
        type: string
      target_date:
        type: string
      owner:
        type: string
      status:
        type: status
      priority:
        type: priority

  # Planning Layer
  epic:
    label: Epic
    aliases: [ep]
    id_patterns: ["EPIC-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      target_release:
        type: string
      status:
        type: status
      priority:
        type: priority

  feature:
    label: Feature
    aliases: [feat]
    id_patterns: ["FEAT-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      acceptance_criteria:
        type: string
      status:
        type: work_status
      priority:
        type: priority
      estimate:
        type: string  # Story points, t-shirt size, or time

  milestone:
    label: Milestone
    aliases: [ms]
    id_patterns: ["MS-"]
    properties:
      title:
        type: string
        required: true
      target_date:
        type: string
      actual_date:
        type: string
      status:
        type: status

  # Execution Layer
  task:
    label: Task
    aliases: [tsk]
    id_patterns: ["TASK-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      assignee:
        type: string
      status:
        type: work_status
      estimate:
        type: string
      actual:
        type: string

  bug:
    label: Bug
    aliases: [bug]
    id_patterns: ["BUG-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      severity:
        type: enum
        values: [blocker, critical, major, minor, trivial]
      found_in:
        type: string
      assignee:
        type: string
      status:
        type: work_status

  # Decision Layer
  decision:
    label: Decision
    aliases: [dec]
    id_patterns: ["DEC-"]
    properties:
      title:
        type: string
        required: true
      context:
        type: string
      options_considered:
        type: string
      chosen_option:
        type: string
      rationale:
        type: string
      consequences:
        type: string
      decided_by:
        type: string
      decided_date:
        type: string
      status:
        type: decision_status

  # Risk Layer
  risk:
    label: Risk
    aliases: [rsk]
    id_patterns: ["RISK-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      likelihood:
        type: likelihood
      impact:
        type: impact
      exposure:
        type: priority  # Calculated from likelihood × impact
      mitigation_strategy:
        type: string
      contingency_plan:
        type: string
      owner:
        type: string
      status:
        type: risk_status

  issue:
    label: Issue
    aliases: [iss]
    id_patterns: ["ISS-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      impact:
        type: string
      resolution:
        type: string
      owner:
        type: string
      status:
        type: enum
        values: [open, investigating, resolving, resolved, accepted]
      raised_date:
        type: string
      resolved_date:
        type: string

  # Stakeholder Layer
  stakeholder:
    label: Stakeholder
    aliases: [stk]
    id_patterns: ["STK-"]
    properties:
      name:
        type: string
        required: true
      role:
        type: string
      organization:
        type: string
      influence:
        type: enum
        values: [high, medium, low]
      interest:
        type: enum
        values: [high, medium, low]
      communication_preference:
        type: string
      engagement_approach:
        type: string

  # Knowledge Layer
  meeting:
    label: Meeting
    aliases: [mtg]
    id_patterns: ["MTG-"]
    properties:
      title:
        type: string
        required: true
      date:
        type: string
      attendees:
        type: string
      meeting_type:
        type: enum
        values: [standup, planning, review, retrospective, steering, workshop, ad_hoc]

  retrospective:
    label: Retrospective
    aliases: [retro]
    id_patterns: ["RETRO-"]
    properties:
      title:
        type: string
        required: true
      date:
        type: string
      what_went_well:
        type: string
      what_needs_improvement:
        type: string
      team:
        type: string

  improvement:
    label: Improvement
    aliases: [imp]
    id_patterns: ["IMP-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      owner:
        type: string
      status:
        type: work_status
      expected_benefit:
        type: string

relations:
  # Goal decomposition
  contributesTo:
    label: contributes to
    description: Lower-level items contribute to higher-level goals
    from: [epic, feature]
    to: [goal]
    inverse:
      name: achievedThrough
      label: achieved through

  partOfEpic:
    label: part of epic
    description: A feature belongs to an epic
    from: [feature]
    to: [epic]
    inverse:
      name: contains
      label: contains

  # Task relationships
  implementedBy:
    label: implemented by
    description: A feature is implemented by tasks
    from: [feature]
    to: [task]
    inverse:
      name: implements
      label: implements

  subtaskOf:
    label: subtask of
    description: A task is a subtask of another
    from: [task]
    to: [task]
    inverse:
      name: hasSubtask
      label: has subtask

  fixes:
    label: fixes
    description: A task fixes a bug
    from: [task]
    to: [bug]
    inverse:
      name: fixedBy
      label: fixed by

  # Dependency relationships
  blockedBy:
    label: blocked by
    description: Work is blocked by other work or issues
    from: [task, feature, epic]
    to: [task, feature, issue, risk]
    inverse:
      name: blocks
      label: blocks

  dependsOn:
    label: depends on
    description: Work depends on other work being completed
    from: [task, feature, epic]
    to: [task, feature]
    inverse:
      name: dependencyOf
      label: dependency of

  # Milestone relationships
  targetedFor:
    label: targeted for
    description: Work is targeted for a milestone
    from: [feature, epic]
    to: [milestone]
    inverse:
      name: includes
      label: includes

  # Decision relationships
  affects:
    label: affects
    description: A decision affects work items
    from: [decision]
    to: [feature, epic, task]
    inverse:
      name: affectedBy
      label: affected by

  decidedIn:
    label: decided in
    description: A decision was made in a meeting
    from: [decision]
    to: [meeting]
    inverse:
      name: produced
      label: produced

  supersedes:
    label: supersedes
    description: A decision supersedes a previous decision
    from: [decision]
    to: [decision]
    inverse:
      name: supersededBy
      label: superseded by

  # Risk relationships
  threatens:
    label: threatens
    description: A risk threatens a goal, feature, or milestone
    from: [risk]
    to: [goal, feature, milestone]
    inverse:
      name: threatenedBy
      label: threatened by

  mitigatedBy:
    label: mitigated by
    description: A risk is mitigated by a task or decision
    from: [risk]
    to: [task, decision]
    inverse:
      name: mitigates
      label: mitigates

  becameIssue:
    label: became issue
    description: A risk materialized into an issue
    from: [risk]
    to: [issue]
    inverse:
      name: originatedFrom
      label: originated from

  # Issue relationships
  resolvedBy:
    label: resolved by
    description: An issue is resolved by a task or decision
    from: [issue]
    to: [task, decision]
    inverse:
      name: resolves
      label: resolves

  # Stakeholder relationships
  ownedBy:
    label: owned by
    description: A goal or feature is owned by a stakeholder
    from: [goal, feature, epic]
    to: [stakeholder]
    target_max: 1  # Each item has one owner
    inverse:
      name: owns
      label: owns

  interestedIn:
    label: interested in
    description: A stakeholder is interested in items
    from: [stakeholder]
    to: [goal, feature, milestone]
    inverse:
      name: hasStakeholder
      label: has stakeholder

  consulted:
    label: consulted
    description: A stakeholder was consulted for a decision
    from: [decision]
    to: [stakeholder]
    inverse:
      name: consultedFor
      label: consulted for

  # Meeting relationships
  attended:
    label: attended
    description: A stakeholder attended a meeting
    from: [stakeholder]
    to: [meeting]
    inverse:
      name: attendedBy
      label: attended by

  discussed:
    label: discussed
    description: A meeting discussed items
    from: [meeting]
    to: [feature, risk, issue, decision]
    inverse:
      name: discussedIn
      label: discussed in

  # Retrospective relationships
  produces:
    label: produces
    description: A retrospective produces improvements
    from: [retrospective]
    to: [improvement]
    inverse:
      name: identifiedIn
      label: identified in

  improvesProcess:
    label: improves process
    description: An improvement enhances how we work
    from: [improvement]
    to: [goal]  # Meta: improving toward process goals
    inverse:
      name: improvedBy
      label: improved by
```

## Example Traceability Chains

### Goal to Task
```
Goal: Increase customer retention by 15% (GOAL-001)
    ↑ contributesTo
Epic: Self-service account management (EPIC-003)
    ↑ partOfEpic
Feature: Password reset flow (FEAT-012)
    ↑ implementedBy
Task: Implement email verification (TASK-045)
```

### Risk to Resolution
```
Risk: Third-party API deprecation (RISK-007)
    ↓ threatens
Feature: Payment processing (FEAT-008)
    ↓ blockedBy
Issue: Stripe API v2 sunset notice (ISS-003)
    ↓ resolvedBy
Task: Migrate to Stripe API v3 (TASK-089)
```

### Decision Context
```
Decision: Use GraphQL over REST (DEC-015)
    ↓ affects
Feature: Mobile API (FEAT-022)
    ↓ decidedIn
Meeting: Architecture review 2024-03-15 (MTG-078)
    ↓ consulted
Stakeholder: Mobile team lead (STK-004)
```

### Retrospective to Improvement
```
Retrospective: Sprint 23 retro (RETRO-023)
    ↓ produces
Improvement: Add definition of done checklist (IMP-008)
    ↓ improvesProcess
Goal: Reduce escaped defects (GOAL-012)
```

## Analysis Queries

Questions project managers can answer:

- **Orphan work**: "Which tasks don't trace back to any goal?"
- **Risk exposure**: "Which milestones are threatened by high-impact risks?"
- **Decision impact**: "What features are affected by decisions made this quarter?"
- **Stakeholder mapping**: "Who needs to be informed about changes to EPIC-003?"
- **Blocked work**: "What's blocking progress on Milestone 2?"
- **Improvement tracking**: "Which retrospective improvements are still open?"
- **Meeting follow-up**: "What decisions and action items came from last week's meetings?"
- **Coverage**: "Which goals have no contributing features?"

## Value Proposition

| Traditional Approach | With Rela |
|---------------------|-----------|
| Requirements in Jira, decisions in Confluence | Unified model with explicit relationships |
| "Why did we decide X?" - hunt through history | Decisions linked to context and rationale |
| Manual status roll-up | Status computed from actual work state |
| Risk register in spreadsheet, disconnected | Risks linked to what they threaten |
| Stakeholder list outdated | Stakeholders linked to what they care about |
| Retro actions forgotten | Improvements tracked to completion |
| Impact analysis is guesswork | Trace dependencies to assess change impact |
| Project knowledge leaves with people | Structured knowledge preserved in docs |

## Workflow Integration

This metamodel supports hybrid workflows:

- **Sprint planning**: Filter features by status, check dependencies
- **Standup**: Show blocked items, highlight risks approaching
- **Steering committee**: Report by goal, show risk exposure
- **Retrospective**: Create improvements, link to what they address
- **Handover**: Navigate full context for any work item
