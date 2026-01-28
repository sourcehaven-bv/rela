# Tutorial: Hybrid Project Management with Rela

This comprehensive tutorial walks you through implementing a complete project
management documentation system using rela. You'll learn how to track goals,
features, tasks, decisions, risks, and stakeholders with full traceability
across all project artifacts.

## What You'll Build

By the end of this tutorial, you'll have a functioning project management
system with:

- Strategic goals linked to measurable outcomes
- Epics and features decomposed from goals
- Tasks implementing features with dependency tracking
- Decisions with context, rationale, and impact tracing
- Risks linked to what they threaten
- Stakeholders mapped to what they care about
- Retrospectives producing tracked improvements
- Analysis tools to identify gaps and assess impact

## Prerequisites

- rela installed (`go install github.com/Sourcehaven-BV/rela/cmd/rela@latest`)
- Basic familiarity with rela concepts (see [Getting Started](../getting-started.md))

## Part 1: Project Setup

### Initialize the Project

```bash
mkdir my-project
cd my-project
rela init
```

### Create the Project Management Metamodel

Replace the default `metamodel.yaml` with this project management configuration:

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
        type: string

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
        type: priority
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
        values:
          [standup, planning, review, retrospective, steering, workshop, ad_hoc]

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
    inverse: achievedThrough

  partOfEpic:
    label: part of epic
    description: A feature belongs to an epic
    from: [feature]
    to: [epic]
    inverse: contains

  # Task relationships
  implementedBy:
    label: implemented by
    description: A feature is implemented by tasks
    from: [feature]
    to: [task]
    inverse: implements

  subtaskOf:
    label: subtask of
    description: A task is a subtask of another
    from: [task]
    to: [task]
    inverse: hasSubtask

  fixes:
    label: fixes
    description: A task fixes a bug
    from: [task]
    to: [bug]
    inverse: fixedBy

  # Dependency relationships
  blockedBy:
    label: blocked by
    description: Work is blocked by other work or issues
    from: [task, feature, epic]
    to: [task, feature, issue, risk]
    inverse: blocks

  dependsOn:
    label: depends on
    description: Work depends on other work being completed
    from: [task, feature, epic]
    to: [task, feature]
    inverse: dependencyOf

  # Milestone relationships
  targetedFor:
    label: targeted for
    description: Work is targeted for a milestone
    from: [feature, epic]
    to: [milestone]
    inverse: includes

  # Decision relationships
  affects:
    label: affects
    description: A decision affects work items
    from: [decision]
    to: [feature, epic, task]
    inverse: affectedBy

  decidedIn:
    label: decided in
    description: A decision was made in a meeting
    from: [decision]
    to: [meeting]
    inverse: produced

  supersedes:
    label: supersedes
    description: A decision supersedes a previous decision
    from: [decision]
    to: [decision]
    inverse: supersededBy

  # Risk relationships
  threatens:
    label: threatens
    description: A risk threatens a goal, feature, or milestone
    from: [risk]
    to: [goal, feature, milestone]
    inverse: threatenedBy

  mitigatedBy:
    label: mitigated by
    description: A risk is mitigated by a task or decision
    from: [risk]
    to: [task, decision]
    inverse: mitigates

  becameIssue:
    label: became issue
    description: A risk materialized into an issue
    from: [risk]
    to: [issue]
    inverse: originatedFrom

  # Issue relationships
  resolvedBy:
    label: resolved by
    description: An issue is resolved by a task or decision
    from: [issue]
    to: [task, decision]
    inverse: resolves

  # Stakeholder relationships
  ownedBy:
    label: owned by
    description: A goal or feature is owned by a stakeholder
    from: [goal, feature, epic]
    to: [stakeholder]
    target_max: 1
    inverse: owns

  interestedIn:
    label: interested in
    description: A stakeholder is interested in items
    from: [stakeholder]
    to: [goal, feature, milestone]
    inverse: hasStakeholder

  consulted:
    label: consulted
    description: A stakeholder was consulted for a decision
    from: [decision]
    to: [stakeholder]
    inverse: consultedFor

  # Meeting relationships
  attended:
    label: attended
    description: A stakeholder attended a meeting
    from: [stakeholder]
    to: [meeting]
    inverse: attendedBy

  discussed:
    label: discussed
    description: A meeting discussed items
    from: [meeting]
    to: [feature, risk, issue, decision]
    inverse: discussedIn

  # Retrospective relationships
  produces:
    label: produces
    description: A retrospective produces improvements
    from: [retrospective]
    to: [improvement]
    inverse: identifiedIn

  improvesProcess:
    label: improves process
    description: An improvement enhances how we work
    from: [improvement]
    to: [goal]
    inverse: improvedBy
```

Save this as `metamodel.yaml` in your project root.

## Part 2: Setting Up Strategic Goals

Every project should start with clear goals. Let's create a few strategic
goals for an example e-commerce platform modernization project.

### Create Your First Goal

```bash
rela create goal -t "Increase customer retention by 15%"
```

This creates `entities/goals/GOAL-001.md`. Let's add more detail:

```bash
rela update GOAL-001 \
  -P "description=Reduce churn and increase repeat purchases through improved user experience" \
  -P "success_criteria=Customer retention rate improves from 45% to 60% YoY" \
  -P "target_date=2024-Q4" \
  -P "priority=critical" \
  -P "status=active"
```

### Create Additional Goals

```bash
rela create goal -t "Reduce time-to-market by 30%"
rela update GOAL-002 \
  -P "description=Accelerate feature delivery through improved tooling and processes" \
  -P "success_criteria=Average feature cycle time drops from 6 weeks to 4 weeks" \
  -P "target_date=2024-Q3" \
  -P "priority=high" \
  -P "status=active"

rela create goal -t "Achieve 99.9% platform uptime"
rela update GOAL-003 \
  -P "description=Improve reliability to support business growth" \
  -P "success_criteria=Monthly uptime consistently exceeds 99.9%" \
  -P "target_date=2024-Q2" \
  -P "priority=high" \
  -P "status=active"
```

### View Your Goals

```bash
rela list goal
```

Or use the interactive TUI:

```bash
rela tui
```

Navigate to Goals to see all strategic objectives.

## Part 3: Creating the Planning Layer

### Create Epics

Epics are large bodies of work that contribute to goals:

```bash
rela create epic -t "Self-service Account Management"
rela update EPIC-001 \
  -P "description=Enable customers to manage their accounts without support intervention" \
  -P "target_release=v2.0" \
  -P "priority=high" \
  -P "status=active"

rela create epic -t "Mobile App Redesign"
rela update EPIC-002 \
  -P "description=Complete redesign of mobile shopping experience" \
  -P "target_release=v2.1" \
  -P "priority=high" \
  -P "status=active"

rela create epic -t "Infrastructure Reliability"
rela update EPIC-003 \
  -P "description=Improve system reliability through better monitoring and redundancy" \
  -P "target_release=v2.0" \
  -P "priority=critical" \
  -P "status=active"
```

### Link Epics to Goals

```bash
rela link EPIC-001 contributesTo GOAL-001
rela link EPIC-002 contributesTo GOAL-001
rela link EPIC-003 contributesTo GOAL-003
```

### Create Features

Features are specific deliverables within an epic:

```bash
# Self-service account features
rela create feature -t "Password Reset Flow"
rela update FEAT-001 \
  -P "description=Allow users to reset passwords via email or SMS" \
  -P "acceptance_criteria=Users can reset password within 2 minutes" \
  -P "estimate=5 points" \
  -P "priority=high" \
  -P "status=ready"

rela create feature -t "Profile Management"
rela update FEAT-002 \
  -P "description=Users can update profile info, preferences, and communication settings" \
  -P "estimate=8 points" \
  -P "priority=medium" \
  -P "status=backlog"

rela create feature -t "Order History Dashboard"
rela update FEAT-003 \
  -P "description=View past orders with filtering and search" \
  -P "estimate=8 points" \
  -P "priority=medium" \
  -P "status=backlog"

# Mobile app features
rela create feature -t "Mobile Checkout Redesign"
rela update FEAT-004 \
  -P "description=Streamlined mobile checkout with Apple Pay and Google Pay" \
  -P "estimate=13 points" \
  -P "priority=high" \
  -P "status=backlog"

# Infrastructure features
rela create feature -t "Automated Failover"
rela update FEAT-005 \
  -P "description=Automatic failover to backup systems on primary failure" \
  -P "estimate=13 points" \
  -P "priority=critical" \
  -P "status=in_progress"
```

### Link Features to Epics

```bash
rela link FEAT-001 partOfEpic EPIC-001
rela link FEAT-002 partOfEpic EPIC-001
rela link FEAT-003 partOfEpic EPIC-001
rela link FEAT-004 partOfEpic EPIC-002
rela link FEAT-005 partOfEpic EPIC-003
```

### Create Milestones

```bash
rela create milestone -t "Phase 1: Foundation"
rela update MS-001 \
  -P "target_date=2024-03-31" \
  -P "status=active"

rela create milestone -t "Phase 2: Self-Service Launch"
rela update MS-002 \
  -P "target_date=2024-06-30" \
  -P "status=draft"

rela create milestone -t "Phase 3: Mobile Relaunch"
rela update MS-003 \
  -P "target_date=2024-09-30" \
  -P "status=draft"
```

### Link Features to Milestones

```bash
rela link FEAT-005 targetedFor MS-001
rela link FEAT-001 targetedFor MS-002
rela link FEAT-002 targetedFor MS-002
rela link FEAT-003 targetedFor MS-002
rela link FEAT-004 targetedFor MS-003
```

## Part 4: Execution Layer - Tasks and Bugs

### Create Tasks

```bash
# Tasks for Password Reset Flow
rela create task -t "Implement email verification service"
rela update TASK-001 \
  -P "description=Build service to send and verify email tokens" \
  -P "assignee=alice@example.com" \
  -P "estimate=2 days" \
  -P "status=in_progress"

rela create task -t "Create password reset UI"
rela update TASK-002 \
  -P "description=Frontend forms for password reset flow" \
  -P "assignee=bob@example.com" \
  -P "estimate=3 days" \
  -P "status=ready"

rela create task -t "Add SMS verification option"
rela update TASK-003 \
  -P "description=Integrate Twilio for SMS-based verification" \
  -P "assignee=alice@example.com" \
  -P "estimate=2 days" \
  -P "status=backlog"

# Tasks for Automated Failover
rela create task -t "Set up health check endpoints"
rela update TASK-004 \
  -P "description=Add /health endpoints to all services" \
  -P "assignee=charlie@example.com" \
  -P "estimate=1 day" \
  -P "status=done"

rela create task -t "Configure load balancer failover"
rela update TASK-005 \
  -P "description=Set up automatic failover in AWS ALB" \
  -P "assignee=charlie@example.com" \
  -P "estimate=2 days" \
  -P "status=in_progress"

rela create task -t "Implement database replication"
rela update TASK-006 \
  -P "description=Set up PostgreSQL streaming replication" \
  -P "assignee=charlie@example.com" \
  -P "estimate=3 days" \
  -P "status=ready"
```

### Link Tasks to Features

```bash
rela link FEAT-001 implementedBy TASK-001
rela link FEAT-001 implementedBy TASK-002
rela link FEAT-001 implementedBy TASK-003
rela link FEAT-005 implementedBy TASK-004
rela link FEAT-005 implementedBy TASK-005
rela link FEAT-005 implementedBy TASK-006
```

### Create Dependencies

```bash
# TASK-002 depends on TASK-001 (need email service before UI)
rela link TASK-002 dependsOn TASK-001

# TASK-003 depends on TASK-001 (reuse verification patterns)
rela link TASK-003 dependsOn TASK-001

# TASK-005 depends on TASK-004 (need health checks before failover)
rela link TASK-005 dependsOn TASK-004

# TASK-006 depends on TASK-005 (database after load balancer)
rela link TASK-006 dependsOn TASK-005
```

### Track a Bug

```bash
rela create bug -t "Password reset emails marked as spam"
rela update BUG-001 \
  -P "description=Reset emails going to spam folder for Gmail users" \
  -P "severity=major" \
  -P "found_in=v1.9.2" \
  -P "assignee=alice@example.com" \
  -P "status=in_progress"

# Create a fix task
rela create task -t "Fix email deliverability issues"
rela update TASK-007 \
  -P "description=Configure SPF, DKIM, and DMARC records" \
  -P "assignee=alice@example.com" \
  -P "estimate=1 day" \
  -P "status=in_progress"

rela link TASK-007 fixes BUG-001
```

## Part 5: Decision Management

### Record an Architectural Decision

```bash
rela create decision -t "Use GraphQL for Mobile API"
rela update DEC-001 \
  -P "context=Mobile app needs efficient data fetching with bandwidth constraints" \
  -P "options_considered=1. REST API, 2. GraphQL, 3. gRPC" \
  -P "chosen_option=GraphQL" \
  -P "rationale=Allows mobile to request exactly needed fields, reducing bandwidth by ~40%" \
  -P "consequences=Team needs GraphQL training; adds complexity to backend" \
  -P "decided_by=Architecture Team" \
  -P "decided_date=2024-01-15" \
  -P "status=decided"
```

### Link Decision to Affected Items

```bash
rela link DEC-001 affects FEAT-004
rela link DEC-001 affects EPIC-002
```

### Record Decision in a Meeting

```bash
rela create meeting -t "Architecture Review - Mobile API"
rela update MTG-001 \
  -P "date=2024-01-15" \
  -P "attendees=Alice, Bob, Charlie, Diana" \
  -P "meeting_type=workshop"

rela link DEC-001 decidedIn MTG-001
```

### Another Decision Example

```bash
rela create decision -t "Adopt Kubernetes for container orchestration"
rela update DEC-002 \
  -P "context=Need container orchestration for microservices deployment" \
  -P "options_considered=1. Docker Swarm, 2. Kubernetes, 3. ECS" \
  -P "chosen_option=Kubernetes (EKS)" \
  -P "rationale=Industry standard, strong ecosystem, team has experience" \
  -P "consequences=Higher infrastructure cost; need to manage cluster" \
  -P "decided_by=Platform Team" \
  -P "decided_date=2024-01-20" \
  -P "status=decided"

rela link DEC-002 affects FEAT-005
rela link DEC-002 affects EPIC-003
```

## Part 6: Risk Management

### Identify Risks

```bash
rela create risk -t "Third-party Payment Provider API Changes"
rela update RISK-001 \
  -P "description=Payment provider may deprecate current API version" \
  -P "likelihood=likely" \
  -P "impact=major" \
  -P "exposure=high" \
  -P "mitigation_strategy=Abstract payment integration; monitor deprecation notices" \
  -P "contingency_plan=Maintain ability to switch providers within 2 weeks" \
  -P "owner=alice@example.com" \
  -P "status=mitigating"

rela create risk -t "Key Team Member Departure"
rela update RISK-002 \
  -P "description=Single point of failure in infrastructure knowledge" \
  -P "likelihood=possible" \
  -P "impact=major" \
  -P "exposure=medium" \
  -P "mitigation_strategy=Document all systems; cross-train team members" \
  -P "owner=bob@example.com" \
  -P "status=mitigating"

rela create risk -t "Mobile App Store Rejection"
rela update RISK-003 \
  -P "description=App updates may be rejected by Apple/Google" \
  -P "likelihood=possible" \
  -P "impact=moderate" \
  -P "exposure=medium" \
  -P "mitigation_strategy=Follow guidelines strictly; pre-submit review" \
  -P "owner=bob@example.com" \
  -P "status=identified"
```

### Link Risks to What They Threaten

```bash
rela link RISK-001 threatens FEAT-004
rela link RISK-001 threatens MS-003
rela link RISK-002 threatens FEAT-005
rela link RISK-002 threatens GOAL-003
rela link RISK-003 threatens EPIC-002
rela link RISK-003 threatens MS-003
```

### Create Mitigation Tasks

```bash
rela create task -t "Create payment provider abstraction layer"
rela update TASK-008 \
  -P "description=Build adapter pattern for payment integrations" \
  -P "assignee=alice@example.com" \
  -P "estimate=3 days" \
  -P "status=ready"

rela link RISK-001 mitigatedBy TASK-008
```

### When a Risk Becomes an Issue

```bash
# Risk materialized!
rela create issue -t "Stripe API v2 Sunset Announced"
rela update ISS-001 \
  -P "description=Stripe announced v2 API end-of-life in 6 months" \
  -P "impact=Must migrate before deadline or payments will fail" \
  -P "owner=alice@example.com" \
  -P "status=investigating" \
  -P "raised_date=2024-02-01"

# Link the risk to the issue
rela link RISK-001 becameIssue ISS-001

# Create resolution task
rela create task -t "Migrate to Stripe API v3"
rela update TASK-009 \
  -P "description=Update all payment integrations to use Stripe API v3" \
  -P "assignee=alice@example.com" \
  -P "estimate=5 days" \
  -P "status=ready"

rela link ISS-001 resolvedBy TASK-009
```

## Part 7: Stakeholder Management

### Define Stakeholders

```bash
rela create stakeholder -t "Sarah Johnson"
rela update STK-001 \
  -P "role=VP of Product" \
  -P "organization=Product Team" \
  -P "influence=high" \
  -P "interest=high" \
  -P "communication_preference=Weekly email summary + monthly steering" \
  -P "engagement_approach=Strategic updates; involve in major decisions"

rela create stakeholder -t "Mike Chen"
rela update STK-002 \
  -P "role=Engineering Director" \
  -P "organization=Engineering" \
  -P "influence=high" \
  -P "interest=high" \
  -P "communication_preference=Daily standups + Slack" \
  -P "engagement_approach=Technical deep-dives; involve in architecture"

rela create stakeholder -t "Lisa Park"
rela update STK-003 \
  -P "role=Customer Success Lead" \
  -P "organization=Customer Success" \
  -P "influence=medium" \
  -P "interest=high" \
  -P "communication_preference=Bi-weekly sync + feature demos" \
  -P "engagement_approach=Customer feedback channel; beta testing coordination"

rela create stakeholder -t "David Kim"
rela update STK-004 \
  -P "role=CFO" \
  -P "organization=Finance" \
  -P "influence=high" \
  -P "interest=medium" \
  -P "communication_preference=Monthly business review" \
  -P "engagement_approach=ROI metrics; budget impacts only"
```

### Link Stakeholders to Items

```bash
# Ownership
rela link GOAL-001 ownedBy STK-001
rela link GOAL-003 ownedBy STK-002
rela link EPIC-001 ownedBy STK-001
rela link EPIC-003 ownedBy STK-002

# Interest
rela link STK-003 interestedIn GOAL-001
rela link STK-003 interestedIn FEAT-001
rela link STK-004 interestedIn GOAL-001
rela link STK-004 interestedIn MS-002

# Decision consultation
rela link DEC-001 consulted STK-002
rela link DEC-002 consulted STK-002
```

### Track Meeting Attendance

```bash
rela link STK-001 attended MTG-001
rela link STK-002 attended MTG-001
```

## Part 8: Knowledge Management

### Record a Retrospective

```bash
rela create retrospective -t "Sprint 23 Retrospective"
rela update RETRO-001 \
  -P "date=2024-02-15" \
  -P "team=Platform Team" \
  -P "what_went_well=Automated deployments saved time; good team collaboration" \
  -P "what_needs_improvement=Too many meetings; unclear priorities mid-sprint"
```

### Create Improvements from Retro

```bash
rela create improvement -t "Implement no-meeting Wednesdays"
rela update IMP-001 \
  -P "description=Reserve Wednesdays for focused development work" \
  -P "owner=bob@example.com" \
  -P "status=in_progress" \
  -P "expected_benefit=Increase deep work time by 20%"

rela create improvement -t "Add priority labels to sprint items"
rela update IMP-002 \
  -P "description=Clear P0/P1/P2 labels on all sprint items" \
  -P "owner=alice@example.com" \
  -P "status=done" \
  -P "expected_benefit=Reduce priority confusion"

rela link RETRO-001 produces IMP-001
rela link RETRO-001 produces IMP-002
```

### Link Improvements to Goals

```bash
rela link IMP-001 improvesProcess GOAL-002
rela link IMP-002 improvesProcess GOAL-002
```

## Part 9: Analysis and Reporting

### Check for Orphan Work

Find tasks that don't trace back to any goal:

```bash
rela analyze orphans
```

### Trace Dependencies

Trace everything related to a goal:

```bash
rela trace from GOAL-001
```

This shows the full hierarchy: Goal → Epics → Features → Tasks

Trace what's needed for a specific task:

```bash
rela trace to TASK-001
```

Find the path between any two items:

```bash
rela trace path GOAL-001 TASK-001
```

### Find Blocked Items

```bash
rela list task --filter "status=blocked"
rela list feature --filter "status=blocked"
```

### View Risk Exposure

```bash
rela list risk --filter "exposure=high"
rela list risk --filter "status=occurred"
```

### Generate Reports

Export data for reporting:

```bash
# Export all items as JSON
rela export json --output project-data.json

# Export as CSV for spreadsheets
rela export csv --output project-data.csv

# Generate dependency graph
rela export dot --output dependencies.dot
dot -Tpng dependencies.dot -o dependencies.png
```

## Part 10: Using the TUI

The Terminal User Interface provides an interactive way to navigate your
project:

```bash
rela tui
```

### Key Navigation

- **Arrow keys**: Navigate lists
- **Enter**: View details / select
- **Tab**: Switch between panes
- **c**: Create new entity
- **l**: Create link from current entity
- **d**: Delete current entity
- **e**: Edit current entity
- **s**: Search
- **g**: View relationship graph
- **?**: Help

### TUI Workflows

**Daily Standup:**

1. Open TUI
2. Navigate to Tasks
3. Filter by `status=in_progress`
4. Review each task, update status as needed
5. Check for blocked items

**Sprint Planning:**

1. Navigate to Features
2. Filter by `status=backlog`
3. Review each feature
4. Create tasks for features selected for sprint
5. Link tasks to features

**Risk Review:**

1. Navigate to Risks
2. Filter by `status=identified` or `exposure=high`
3. Review each risk
4. Update status or create mitigation tasks

## Best Practices

### 1. Start with Goals

Always create goals first, then decompose into epics, features, and tasks.
This ensures all work traces to business value.

### 2. Link Everything

Create relationships as you create entities. An unlinked entity is an orphan
waiting to happen.

### 3. Update Status Regularly

Keep status fields current. Stale data undermines trust in the system.

### 4. Document Decisions

Every significant choice should be a decision record with context and
rationale. Future you will thank present you.

### 5. Use Analysis Before Meetings

Before steering committees or planning sessions:

```bash
rela analyze orphans
rela list risk --filter "exposure=high"
rela trace from GOAL-001
```

### 7. Export for Stakeholders

Non-technical stakeholders may prefer spreadsheets:

```bash
rela export csv --output weekly-report.csv
```

## Common Queries

**What's blocking the milestone?**

```bash
rela trace to MS-002
rela list feature --filter "status=blocked"
```

**Who cares about this feature?**

```bash
rela show FEAT-001
# Look at "has stakeholder" relationships
```

**What decisions affect this epic?**

```bash
rela show EPIC-002
# Look at "affected by" relationships
```

**What risks threaten our timeline?**

```bash
rela list risk --filter "status!=resolved"
```

**What came out of the last retro?**

```bash
rela show RETRO-001
rela list improvement --filter "status!=done"
```

## Next Steps

You now have a fully functional project management system. Consider:

1. **Customize the metamodel** for your specific needs
2. **Integrate with CI/CD** to auto-update task status
3. **Set up regular analysis** as part of your workflow
4. **Export data** to dashboards or reporting tools

For more information, see:

- [Metamodel Reference](../reference/metamodel.md)
- [CLI Reference](../reference/cli.md)
- [Analysis Commands](../reference/analysis.md)
