# Scenario: ISO 27001 Information Security Management System

## Background

Organizations seeking ISO 27001 certification must implement and maintain an
Information Security Management System (ISMS). This involves extensive
documentation: policies, procedures, risk assessments, control implementations,
and evidence of continuous improvement. Traditionally, this documentation lives
in scattered Word documents, SharePoint sites, or expensive GRC tools—making it
hard to maintain, trace, and audit.

The core challenge is **traceability**: auditors want to see how business risks
connect to security controls, how controls are implemented, and what evidence
exists. Without clear traceability, organizations spend weeks preparing for
audits, hunting for documents, and reconstructing rationale for past decisions.

## Context

An ISMS requires several interconnected documentation layers:

1. **Scope & Context** - Organizational context, interested parties, ISMS
   boundaries
2. **Risk Management** - Asset inventory, threat identification, risk
   assessments, treatment plans
3. **Controls** - Security controls from Annex A (or custom), their
   applicability, implementation status
4. **Policies & Procedures** - Governing documents that mandate behaviors
5. **Evidence** - Audit logs, training records, configuration snapshots proving
   control effectiveness
6. **Continual Improvement** - Nonconformities, corrective actions, management
   review outputs

The relationships between these are crucial:

- Risks must be treated by controls
- Controls must be implemented via procedures
- Procedures must have evidence of execution
- Nonconformities must trace back to the control that failed

## Goals

### Primary Goals

1. **Audit Readiness** - Generate a complete, consistent view of the ISMS for
   internal and external auditors
2. **Traceability** - Trace any control back to its justifying risk, and forward
   to its implementing evidence
3. **Gap Analysis** - Identify controls without implementation, risks without
   treatment, evidence gaps
4. **Living Documentation** - Keep the ISMS current as the organization evolves,
   not a snapshot that decays

### Secondary Goals

5. **Statement of Applicability (SoA)** - Auto-generate the SoA from control
   entities and their applicability status
6. **Risk Treatment Plan** - Generate risk treatment views from risk-control
   relationships
7. **Management Reporting** - Dashboards showing ISMS health, outstanding
   actions, coverage metrics
8. **Version History** - Git-based audit trail showing who changed what, when,
   and why

## Proposed Metamodel

```yaml
version: "1.0"
namespace: "https://example.org/ontology/isms#"

types:
  status:
    values: [draft, active, under_review, retired]
    default: draft

  risk_level:
    values: [critical, high, medium, low, negligible]

  treatment_status:
    values: [identified, planned, implementing, implemented, accepted]
    default: identified

  control_applicability:
    values: [applicable, not_applicable, partially_applicable]
    default: applicable

  evidence_type:
    values: [document, log, screenshot, attestation, automated_check]

entities:
  # Assets and Risks
  asset:
    label: Information Asset
    aliases: [ast]
    id_patterns: ["AST-"]
    properties:
      title:
        type: string
        required: true
      owner:
        type: string
      classification:
        type: enum
        values: [public, internal, confidential, restricted]

  risk:
    label: Risk
    aliases: [rsk]
    id_patterns: ["RSK-"]
    properties:
      title:
        type: string
        required: true
      threat:
        type: string
      vulnerability:
        type: string
      likelihood:
        type: risk_level
      impact:
        type: risk_level
      inherent_risk:
        type: risk_level
      residual_risk:
        type: risk_level
      treatment_status:
        type: treatment_status

  # Controls (Annex A aligned)
  control:
    label: Control
    aliases: [ctrl]
    id_patterns: ["A.", "CTRL-"]  # A.5.1 for Annex A, CTRL- for custom
    properties:
      title:
        type: string
        required: true
      objective:
        type: string
      applicability:
        type: control_applicability
      justification:
        type: string  # Required when not_applicable
      implementation_status:
        type: enum
        values: [not_started, in_progress, implemented, not_applicable]

  # Policies and Procedures
  policy:
    label: Policy
    aliases: [pol]
    id_patterns: ["POL-"]
    properties:
      title:
        type: string
        required: true
      version:
        type: string
      effective_date:
        type: string
      review_date:
        type: string
      owner:
        type: string
      status:
        type: status

  procedure:
    label: Procedure
    aliases: [proc]
    id_patterns: ["PROC-"]
    properties:
      title:
        type: string
        required: true
      owner:
        type: string
      status:
        type: status

  # Evidence
  evidence:
    label: Evidence
    aliases: [evd]
    id_patterns: ["EVD-"]
    properties:
      title:
        type: string
        required: true
      evidence_type:
        type: evidence_type
      collected_date:
        type: string
      valid_until:
        type: string
      location:
        type: string  # Path, URL, or reference to external system

  # Continual Improvement
  nonconformity:
    label: Nonconformity
    aliases: [nc]
    id_patterns: ["NC-"]
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      identified_date:
        type: string
      source:
        type: enum
        values: [internal_audit, external_audit, incident, management_review, observation]
      severity:
        type: enum
        values: [major, minor, observation]
      status:
        type: enum
        values: [open, investigating, correcting, closed, verified]

  corrective_action:
    label: Corrective Action
    aliases: [ca]
    id_patterns: ["CA-"]
    properties:
      title:
        type: string
        required: true
      owner:
        type: string
      due_date:
        type: string
      completion_date:
        type: string
      status:
        type: enum
        values: [planned, in_progress, completed, verified, ineffective]

relations:
  # Risk relationships
  threatens:
    label: threatens
    description: A risk threatens an asset
    from: [risk]
    to: [asset]
    inverse:
      name: threatenedBy
      label: threatened by

  treatedBy:
    label: treated by
    description: A risk is treated by a control
    from: [risk]
    to: [control]
    source_min: 1  # Every risk must have at least one treatment
    inverse:
      name: treats
      label: treats

  # Control relationships
  implementedBy:
    label: implemented by
    description: A control is implemented by a procedure
    from: [control]
    to: [procedure]
    inverse:
      name: implements
      label: implements

  mandatedBy:
    label: mandated by
    description: A procedure is mandated by a policy
    from: [procedure]
    to: [policy]
    inverse:
      name: mandates
      label: mandates

  # Evidence relationships
  evidences:
    label: evidences
    description: Evidence demonstrates control effectiveness
    from: [evidence]
    to: [control]
    inverse:
      name: evidencedBy
      label: evidenced by

  supportsProc:
    label: supports
    description: Evidence supports procedure execution
    from: [evidence]
    to: [procedure]
    inverse:
      name: supportedBy
      label: supported by

  # Nonconformity relationships
  affects:
    label: affects
    description: A nonconformity affects a control
    from: [nonconformity]
    to: [control]
    inverse:
      name: affectedBy
      label: affected by

  addressedBy:
    label: addressed by
    description: A nonconformity is addressed by a corrective action
    from: [nonconformity]
    to: [corrective_action]
    source_min: 1  # Every NC must have at least one CA
    inverse:
      name: addresses
      label: addresses

  # Cross-references
  relatesTo:
    label: relates to
    from: [policy, procedure, control]
    to: [policy, procedure, control]
    symmetric: true
```

## Example Traceability Chain

```
Asset: Customer Database (AST-001)
    ↑ threatens
Risk: Unauthorized data access (RSK-012)
    ↓ treatedBy
Control: Access control policy (A.9.1.1)
    ↓ implementedBy
Procedure: User access provisioning (PROC-023)
    ↓ supportedBy
Evidence: Access request tickets Q4 2024 (EVD-089)
```

## Analysis Queries

With this metamodel, auditors and security managers can answer:

- **Coverage**: "Which controls have no implementing procedures?"
- **Evidence gaps**: "Which controls lack recent evidence?"
- **Risk exposure**: "Which high-impact risks have unimplemented controls?"
- **Audit prep**: "Show all evidence for A.9 Access Control family"
- **Orphans**: "Which procedures aren't linked to any control?"
- **Improvement tracking**: "Which corrective actions are overdue?"

## Value Proposition

| Traditional Approach                | With Rela                                      |
| ----------------------------------- | ---------------------------------------------- |
| Scattered documents, no clear links | Single source of truth with explicit relations |
| Manual SoA updates in spreadsheets  | Auto-generated SoA from control entities       |
| Audit prep takes weeks              | Instant traceability queries                   |
| Version history unclear             | Git provides full audit trail                  |
| Evidence hunting during audits      | Evidence linked directly to controls           |
| Stale documentation                 | Living docs updated alongside processes        |
