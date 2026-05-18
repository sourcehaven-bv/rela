<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Concepts

This document explains the core concepts behind traceability and
how rela implements them.

## What is Traceability?

Traceability is the ability to track relationships between
different levels of artifacts—from high-level requirements down to
implementation components.

```text
Why?          What?           How?              Where?
  │             │               │                  │
  ▼             ▼               ▼                  ▼
Requirement → Decision → Solution → Component
```

Traceability answers questions like:

- **Impact analysis**: "If this requirement changes, what else is affected?"
- **Completeness**: "Is every requirement addressed by a design decision?"
- **Rationale**: "Why does this component exist? What decision led to it?"

## Core Entity Types

### Requirements

Requirements describe what the system must do. They can be:

- **Functional** (FR): Features the system provides
- **Non-functional** (NFR): Quality attributes like performance, security

```bash
rela create requirement --title "System must authenticate users via OAuth 2.0"
```

### Decisions

Decisions (also called Architecture Decision Records or ADRs) document
significant choices made during design. They should explain:

- The context and problem
- The decision made
- The rationale (why this option was chosen)
- Consequences

```bash
rela create decision --title "Use JWT tokens for session management"
```

### Solutions

Solutions describe how decisions are implemented at a design level. They bridge
the gap between abstract decisions and concrete components.

```bash
rela create solution --title "Auth service with Redis-backed token storage"
```

### Components

Components are the concrete, deployable artifacts that realize solutions. These
could be:

- Microservices
- Libraries
- Containers
- Infrastructure resources

```bash
rela create component --title "auth-service Docker container"
```

## Relations

Relations create the traceability chain by connecting entities.

### Core Traceability Relations

```text
           addresses           implements          realizes
Decision ──────────→ Requirement    Solution ──────────→ Decision    Component ──────────→ Solution
         ←──────────              ←──────────            ←──────────
         addressedBy              implementedBy          realizedBy
```

| Relation     | Meaning                                        |
| ------------ | ---------------------------------------------- |
| `addresses`  | A decision addresses/responds to a requirement |
| `implements` | A solution implements a decision               |
| `realizes`   | A component realizes a solution                |
| `dependsOn`  | An entity depends on another entity            |

### Building a Trace

```bash
# Create the chain
rela create requirement --title "Users must be authenticated"
rela create decision --title "Implement OAuth 2.0 with PKCE"
rela create solution --title "Auth service using Keycloak"
rela create component --title "keycloak-deployment"

# Link them
rela link DEC-001 addresses REQ-001
rela link SOL-001 implements DEC-001
rela link COMP-001 realizes SOL-001
```

### Tracing the Chain

Forward (downstream): "What implements this requirement?"

```bash
rela trace from REQ-001
```

Backward (upstream): "Why does this component exist?"

```bash
rela trace to COMP-001
```

Find the path:

```bash
rela trace path REQ-001 COMP-001
```

## The Metamodel

The metamodel is your project's schema. It defines:

- What entity types exist
- What properties each type has
- What relations are allowed between types

This is stored in `metamodel.yaml`. See [Metamodel Reference](metamodel.md) for
details.

## Audit Log

Every write rela performs — through any entry point (CLI, MCP, the
data-entry web app, the scheduler, the desktop app) — is recorded as
an append-only JSONL line under `.rela/audit/YYYY-MM-DD.jsonl`. The
log answers "what changed, when, and on whose behalf"; common
questions a user might have:

- **Who changed entity X today?** `jq 'select(.subject.id == "X")' .rela/audit/$(date -u +%Y-%m-%d).jsonl`
- **What did the scheduler do this week?** `cat .rela/audit/*.jsonl | jq 'select(.principal.tool == "scheduler")'`
- **What automation cascaded from my last edit?** Look for records with `triggered_by: "automation:<name>"` near your write.

See [audit-log.md](audit-log.md) for the full record schema, the
`Principal{user, tool}` contract, durability caveats, and operator
concerns (retention, rotation).

## Storage Format

Entities are stored as Markdown files with YAML frontmatter:

```text
entities/
├── requirements/
│   └── REQ-001.md
├── decisions/
│   └── DEC-001.md
└── components/
    └── COMP-001.md
```

Example entity file (`REQ-001.md`):

```markdown
---
id: REQ-001
type: requirement
title: Users must be authenticated
status: accepted
priority: high
---

All users must authenticate before accessing protected resources.

## Acceptance Criteria

- Users can log in with email/password
- Users can use OAuth providers (Google, GitHub)
- Sessions expire after 24 hours of inactivity
```

Relations are also stored as Markdown:

```text
relations/
└── DEC-001--addresses--REQ-001.md
```

This format is:

- **Human-readable**: Easy to review in pull requests
- **Version-controllable**: Works naturally with Git
- **Portable**: No database required

## Quality Analysis

### Cardinality Constraints

Cardinality constraints verify that entities have the required number of
relations. Define constraints in your metamodel:

```yaml
relations:
  addresses:
    from: [decision]
    to: [requirement]
    min_incoming: 1  # Every requirement must be addressed by at least one decision
```

Check constraints with:

```bash
rela analyze cardinality
```

This checks all `min_outgoing`, `max_outgoing`, `min_incoming`, and `max_incoming`
constraints defined on relations.

### Orphan Detection

Orphans are entities with no relations—they're disconnected from the
architecture graph.

```bash
rela analyze orphans
```

Orphans might indicate:

- Forgotten entities that should be linked
- Outdated entities that should be deleted
- Work in progress

### ID Gap Analysis

ID gaps find missing numbers in sequences:

```bash
rela analyze gaps
```

If you have REQ-001, REQ-002, REQ-004, this reports REQ-003 as missing. This
could indicate:

- Deleted entities
- Numbering mistakes

**Note:** Gap analysis only applies to entity types with `id_type: sequential`.
Entity types with `short` (default) or `manual` IDs are excluded since they don't
follow numeric sequences.

### Duplicate Detection

Finds entities with similar titles:

```bash
rela analyze duplicates
```

Duplicates might indicate:

- Redundant requirements
- Entities that should be consolidated

### Cardinality Validation

Cardinality constraints (defined in the metamodel) specify how many relations an
entity should have:

```bash
rela analyze cardinality
```

For example, if the metamodel specifies `min_outgoing: 1` for `addresses`, every
decision must address at least one requirement.

## Best Practices

### Start with Requirements

Begin by documenting your requirements. These are the foundation of
traceability.

### Document Decisions as You Make Them

Don't wait until the end. Document decisions when the context is fresh.

### Use Meaningful IDs

Configure ID patterns that make sense:

- `FR-` for functional requirements
- `NFR-` for non-functional requirements
- `ADR-` for architecture decision records

For some entity types like components or modules, consider using manual IDs
(`id_type: manual` in metamodel) with descriptive names like `auth-service` or
`payment-gateway` instead of auto-generated numbers.

### Run Analysis Regularly

Make `rela analyze all` part of your workflow:

- Before releases
- In CI pipelines
- During architecture reviews

### Keep It Updated

Traceability is only valuable if it's current. When you:

- Add a new component → link it to its solution
- Change a requirement → update affected decisions
- Remove a feature → cascade delete the trace

### Version Control Everything

The Markdown-based storage is designed for Git:

- Review architecture changes in PRs
- Track evolution over time
- Collaborate with your team

## Common Patterns

### Requirement Decomposition

Large requirements can be broken down:

```text
REQ-001 (Parent requirement)
├── REQ-002 (Sub-requirement)
├── REQ-003 (Sub-requirement)
└── REQ-004 (Sub-requirement)
```

Use the `derivedFrom` relation (if configured in your metamodel).

### Component Dependencies

Track which components depend on others:

```bash
rela link COMP-002 dependsOn COMP-001
```

Useful for impact analysis when changing shared components.

### Cross-Cutting Concerns

Some decisions affect multiple requirements:

```bash
rela link DEC-001 addresses REQ-001
rela link DEC-001 addresses REQ-002
rela link DEC-001 addresses REQ-003
```

This is expected and shows how one design choice satisfies multiple needs.
