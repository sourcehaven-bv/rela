---
id: SCN-devops-runbooks
type: scenario
title: "Scenario: DevOps/SRE Runbooks & Infrastructure Operations"
status: published
domain: devops
summary: "DevOps/SRE runbooks and infrastructure operations"
---

## Background

Modern DevOps and SRE teams manage complex, distributed systems where operational knowledge is
critical but often tribal. Runbooks exist as wiki pages, procedures live in team members' heads,
and the connection between infrastructure components, their failure modes, and the procedures to
handle them is implicit at best.

When incidents occur, engineers scramble to find the right runbook, discover it's outdated, or
realize no runbook exists. Post-mortems identify documentation gaps, but the fixes rarely stick
because there's no systematic way to ensure every service has runbooks, every runbook is current,
and every alert links to its procedure.

The "you build it, you run it" DevOps philosophy means development teams own both code and
operations—but without structured operational documentation, this ownership becomes a liability
when team members leave or rotate.

## Context

A DevOps/SRE documentation system needs to capture:

1. **Infrastructure Components** - Services, databases, queues, clusters, cloud resources
2. **Dependencies** - How components interact, what breaks when something fails
3. **Failure Modes** - Known ways things go wrong (high CPU, disk full, connection pool exhausted)
4. **Alerts** - Monitoring rules that fire when failure modes occur
5. **Runbooks** - Step-by-step procedures to diagnose and remediate issues
6. **Playbooks** - Higher-level guides for complex scenarios (incident response, disaster recovery)
7. **SLOs/SLIs** - Service level objectives and indicators that define "healthy"

The key relationships:

- Components have failure modes
- Failure modes trigger alerts
- Alerts link to runbooks
- Runbooks reference components they operate on
- Components depend on other components (cascading impact)
- SLOs define what "healthy" means for a component

## Goals

### Primary Goals

1. **Incident Response Speed** - From alert to runbook in one click
2. **Coverage Visibility** - Know which services lack runbooks before incidents happen
3. **Dependency Mapping** - Understand blast radius when a component fails
4. **Runbook Currency** - Track which runbooks are stale and need review

### Secondary Goals

1. **On-call Onboarding** - New team members can explore the operational landscape
2. **Post-mortem Actions** - Link improvement items to the components/runbooks they improve
3. **Change Impact** - Before deploying, understand what operational docs need updating
4. **SRE Metrics** - Track operational maturity: % services with runbooks, % alerts with procedures

## Proposed Metamodel

```yaml
version: "1.0"
namespace: "https://example.org/ontology/sre#"

types:
  status:
    values: [draft, active, deprecated, archived]
    default: draft

  severity:
    values: [critical, major, minor, warning, info]

  tier:
    values: [tier0, tier1, tier2, tier3] # tier0 = most critical

  runbook_status:
    values: [current, needs_review, outdated, missing]
    default: current

  environment:
    values: [production, staging, development, shared]

entities:
  # Infrastructure Components
  service:
    label: Service
    aliases: [svc]
    id_patterns: ["SVC-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      tier:
        type: tier
      owner_team:
        type: string
      repository:
        type: string
      oncall_rotation:
        type: string

  database:
    label: Database
    aliases: [db]
    id_patterns: ["DB-"]
    properties:
      title:
        type: string
        required: true
      engine:
        type: enum
        values:
          [postgresql, mysql, mongodb, redis, elasticsearch, dynamodb, other]
      tier:
        type: tier
      environment:
        type: environment

  queue:
    label: Message Queue
    aliases: [q, mq]
    id_patterns: ["Q-"]
    properties:
      title:
        type: string
        required: true
      broker:
        type: enum
        values: [rabbitmq, kafka, sqs, pubsub, nats, other]
      tier:
        type: tier

  cluster:
    label: Cluster
    aliases: [k8s, cls]
    id_patterns: ["CLS-"]
    properties:
      title:
        type: string
        required: true
      platform:
        type: enum
        values: [kubernetes, ecs, nomad, swarm, vms]
      environment:
        type: environment
      cloud_provider:
        type: enum
        values: [aws, gcp, azure, on-prem, hybrid]

  # Operational Knowledge
  failure_mode:
    label: Failure Mode
    aliases: [fm]
    id_patterns: ["FM-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      symptoms:
        type: string
      typical_causes:
        type: string
      severity:
        type: severity

  alert:
    label: Alert
    aliases: [alrt]
    id_patterns: ["ALT-"]
    properties:
      title:
        type: string
        required: true
      query:
        type: string # The actual alert query/expression
      threshold:
        type: string
      severity:
        type: severity
      source:
        type: enum
        values: [prometheus, datadog, cloudwatch, pagerduty, grafana, custom]
      pages_oncall:
        type: enum
        values: [yes, no, business_hours_only]

  runbook:
    label: Runbook
    aliases: [rb]
    id_patterns: ["RB-"]
    properties:
      title:
        type: string
        required: true
      summary:
        type: string
      estimated_time:
        type: string # e.g., "5-10 minutes"
      requires_access:
        type: string # e.g., "production SSH, AWS console"
      last_reviewed:
        type: string
      review_status:
        type: runbook_status
      automation_status:
        type: enum
        values: [manual, partially_automated, fully_automated]

  playbook:
    label: Playbook
    aliases: [pb]
    id_patterns: ["PB-"]
    properties:
      title:
        type: string
        required: true
      scenario:
        type: enum
        values:
          [
            incident_response,
            disaster_recovery,
            security_breach,
            capacity_planning,
            maintenance_window,
            rollback,
            other,
          ]
      scope:
        type: string
      last_tested:
        type: string
      status:
        type: status

  # SLOs and Reliability
  slo:
    label: Service Level Objective
    aliases: [slo]
    id_patterns: ["SLO-"]
    properties:
      title:
        type: string
        required: true
      indicator:
        type: string # e.g., "availability", "latency p99"
      target:
        type: string # e.g., "99.9%", "< 200ms"
      window:
        type: string # e.g., "30 days rolling"
      error_budget_policy:
        type: string

  # Improvement Tracking
  postmortem:
    label: Post-mortem
    aliases: [pm]
    id_patterns: ["PM-"]
    properties:
      title:
        type: string
        required: true
      incident_date:
        type: string
      severity:
        type: severity
      duration:
        type: string
      impact:
        type: string
      status:
        type: enum
        values: [draft, reviewed, published, actions_complete]

  action_item:
    label: Action Item
    aliases: [ai]
    id_patterns: ["AI-"]
    properties:
      title:
        type: string
        required: true
      owner:
        type: string
      due_date:
        type: string
      priority:
        type: enum
        values: [p0, p1, p2, p3]
      status:
        type: enum
        values: [open, in_progress, completed, wont_fix]

relations:
  # Dependency graph
  dependsOn:
    label: depends on
    description: A component depends on another component
    from: [service, database, queue]
    to: [service, database, queue, cluster]
    inverse: dependencyOf

  runsOn:
    label: runs on
    description: A component runs on a cluster/platform
    from: [service, database, queue]
    to: [cluster]
    inverse: hosts

  # Failure mode relationships
  hasFailureMode:
    label: has failure mode
    description: A component can fail in this way
    from: [service, database, queue, cluster]
    to: [failure_mode]
    inverse: affectsComponent

  triggers:
    label: triggers
    description: A failure mode triggers an alert
    from: [failure_mode]
    to: [alert]
    inverse: detects

  # Runbook relationships
  remediates:
    label: remediates
    description: A runbook remediates a failure mode
    from: [runbook]
    to: [failure_mode]
    min_outgoing: 1 # Every runbook must address at least one failure mode
    inverse: remediatedBy

  linkedToAlert:
    label: linked to alert
    description: An alert links to a runbook
    from: [alert]
    to: [runbook]
    min_outgoing: 1 # Every alert should have a runbook
    inverse: triggeredBy

  operatesOn:
    label: operates on
    description: A runbook operates on a component
    from: [runbook]
    to: [service, database, queue, cluster]
    inverse: operatedBy

  # Playbook relationships
  includes:
    label: includes
    description: A playbook includes runbooks as steps
    from: [playbook]
    to: [runbook]
    inverse: partOf

  coversScenarioFor:
    label: covers scenario for
    description: A playbook covers a failure scenario for components
    from: [playbook]
    to: [service, cluster]
    inverse: hasPlaybook

  # SLO relationships
  hasSLO:
    label: has SLO
    description: A service has an SLO
    from: [service]
    to: [slo]
    inverse: appliesTo

  violationIndicates:
    label: violation indicates
    description: An alert indicates potential SLO violation
    from: [alert]
    to: [slo]
    inverse: monitoredBy

  # Post-mortem relationships
  investigates:
    label: investigates
    description: A post-mortem investigates a failure mode
    from: [postmortem]
    to: [failure_mode]
    inverse: investigatedIn

  involves:
    label: involves
    description: A post-mortem involves components
    from: [postmortem]
    to: [service, database, queue, cluster]
    inverse: involvedIn

  produces:
    label: produces
    description: A post-mortem produces action items
    from: [postmortem]
    to: [action_item]
    inverse: producedBy

  improves:
    label: improves
    description: An action item improves a runbook/component
    from: [action_item]
    to: [runbook, service, database, queue, cluster, playbook]
    inverse: improvedBy
```

## Example Traceability Chains

### Alert to Resolution

```text
Alert: High API latency (ALT-023)
    ↓ linkedToAlert
Runbook: Investigate API latency (RB-015)
    ↓ operatesOn
Service: API Gateway (SVC-001)
    ↓ dependsOn
Database: User DB (DB-003)
```

### Failure Mode Coverage

```text
Service: Payment Service (SVC-007)
    ↓ hasFailureMode
Failure Mode: Connection pool exhausted (FM-012)
    ↓ triggers
Alert: Payment DB connections high (ALT-045)
    ↓ linkedToAlert
Runbook: Scale payment DB connections (RB-033)
```

### Post-mortem to Improvement

```text
Post-mortem: Black Friday outage 2024 (PM-008)
    ↓ produces
Action Item: Add circuit breaker to payment flow (AI-023)
    ↓ improves
Service: Payment Service (SVC-007)
```

## Analysis Queries

Essential operational questions this enables:

- **Runbook coverage**: "Which tier0 services have failure modes without runbooks?"
- **Alert hygiene**: "Which alerts have no linked runbook?"
- **Dependency blast radius**: "If DB-003 goes down, what services are affected?"
- **Stale docs**: "Which runbooks haven't been reviewed in 6 months?"
- **On-call readiness**: "Show all runbooks for services owned by team-payments"
- **Post-mortem follow-up**: "Which action items are overdue?"
- **SLO coverage**: "Which tier0 services have no SLO defined?"

## Value Proposition

| Traditional Approach            | With Rela                                  |
| ------------------------------- | ------------------------------------------ |
| Wiki pages with broken links    | Structured relations between all artifacts |
| "Ask Sarah, she knows"          | Explicit ownership and linked knowledge    |
| Runbooks found mid-incident     | Alerts link directly to runbooks           |
| Unknown coverage gaps           | Analysis shows exactly what's missing      |
| Stale docs discovered in crisis | Review dates and staleness tracking        |
| Post-mortem actions forgotten   | Actions linked to what they improve        |
| Manual dependency diagrams      | Auto-generated from relations              |
| Onboarding takes months         | Navigable operational knowledge graph      |

## Integration Points

This metamodel can integrate with:

- **Monitoring systems** - Import alerts from Prometheus/Datadog
- **PagerDuty/Opsgenie** - Link alerts to incident management
- **Git repositories** - Runbooks can reference code, code can reference runbooks
- **CI/CD** - Validate runbook existence in deployment pipelines
- **Kubernetes** - Sync service definitions from cluster manifests
