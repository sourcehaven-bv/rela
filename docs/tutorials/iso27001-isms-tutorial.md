# Tutorial: Building an ISO 27001 ISMS with Rela

This comprehensive tutorial walks you through implementing a complete
Information Security Management System (ISMS) using rela. You'll learn how to
model all aspects of ISO 27001 compliance, from risk assessment through audit
preparation.

## What You'll Build

By the end of this tutorial, you'll have a functioning ISMS with:

- Information assets and risk assessments
- Security controls mapped to multiple compliance frameworks
- Policies and procedures with traceability
- Evidence management for audit readiness
- Nonconformity and corrective action tracking
- Analysis tools to identify gaps and coverage

## Prerequisites

- rela installed (`go install github.com/Sourcehaven-BV/rela/cmd/rela@latest`)
- Basic familiarity with rela concepts (see
  [Getting Started](../getting-started.md))
- Understanding of ISO 27001 concepts (helpful but not required)

## Part 1: Project Setup

### Initialize the Project

```bash
mkdir my-isms
cd my-isms
rela init
```

### Create the ISMS Metamodel

Replace the default `metamodel.yaml` with this ISMS-specific configuration:

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

  control:
    label: Control
    aliases: [ctrl]
    id_patterns: ["CTRL-"]
    properties:
      title:
        type: string
        required: true
      objective:
        type: string
      iso27001:
        type: string
        description: "ISO 27001:2022 Annex A reference (e.g., A.5.15)"
      iso27001_2013:
        type: string
        description: "ISO 27001:2013 reference (legacy)"
      nist_csf:
        type: string
        description: "NIST Cybersecurity Framework reference"
      soc2:
        type: string
        description: "SOC 2 Trust Services Criteria reference"
      applicability:
        type: control_applicability
      justification:
        type: string
      implementation_status:
        type: enum
        values: [not_started, in_progress, implemented, not_applicable]

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
        type: date
        format: "2006-01-02"
      review_date:
        type: date
        format: "2006-01-02"
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
        type: date
        format: "2006-01-02"
      valid_until:
        type: date
        format: "2006-01-02"
      location:
        type: string

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
        type: date
        format: "2006-01-02"
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
        type: date
        format: "2006-01-02"
      completion_date:
        type: date
        format: "2006-01-02"
      status:
        type: enum
        values: [planned, in_progress, completed, verified, ineffective]

relations:
  threatens:
    label: threatens
    description: A risk threatens an asset
    from: [risk]
    to: [asset]
    inverse: threatenedBy

  treatedBy:
    label: treated by
    description: A risk is treated by a control
    from: [risk]
    to: [control]
    source_min: 1
    inverse: treats

  implementedBy:
    label: implemented by
    description: A control is implemented by a procedure
    from: [control]
    to: [procedure]
    inverse: implements

  mandatedBy:
    label: mandated by
    description: A procedure is mandated by a policy
    from: [procedure]
    to: [policy]
    inverse: mandates

  evidences:
    label: evidences
    description: Evidence demonstrates control effectiveness
    from: [evidence]
    to: [control]
    inverse: evidencedBy

  supportsProc:
    label: supports
    description: Evidence supports procedure execution
    from: [evidence]
    to: [procedure]
    inverse: supportedBy

  affects:
    label: affects
    description: A nonconformity affects a control
    from: [nonconformity]
    to: [control]
    inverse: affectedBy

  addressedBy:
    label: addressed by
    description: A nonconformity is addressed by a corrective action
    from: [nonconformity]
    to: [corrective_action]
    source_min: 1
    inverse: addresses

  relatesTo:
    label: relates to
    from: [policy, procedure, control]
    to: [policy, procedure, control]
    symmetric: true
```

Sync to load the metamodel:

```bash
rela sync
```

---

## Part 2: Establish the ISMS Foundation (Security Manager Role)

### Create Foundational Policies

Every ISMS starts with top-level policies that set the security direction.

```bash
# Information Security Policy - the cornerstone
rela create policy POL-001 --title "Information Security Policy"

# Access Control Policy
rela create policy POL-002 --title "Access Control Policy"

# Acceptable Use Policy
rela create policy POL-003 --title "Acceptable Use Policy"

# Risk Management Policy
rela create policy POL-004 --title "Risk Management Policy"
```

Update the policy files to add properties. Edit `entities/policies/POL-001.md`:

```markdown
---
id: POL-001
title: Information Security Policy
version: "1.0"
effective_date: 2025-01-01
review_date: 2026-01-01
owner: Chief Information Security Officer
status: active
---

# Information Security Policy

This policy establishes the information security objectives and principles for
the organization...
```

### Verify Setup

```bash
rela list policy
```

Expected output:

```
ID       TYPE     TITLE                        STATUS
POL-001  policy   Information Security Policy  active
POL-002  policy   Access Control Policy        draft
POL-003  policy   Acceptable Use Policy        draft
POL-004  policy   Risk Management Policy       draft
```

---

## Part 3: Asset Inventory and Risk Assessment (Risk Analyst Role)

### Create Information Assets

Identify critical information assets:

```bash
rela create asset AST-001 --title "Customer Database"
rela create asset AST-002 --title "Employee HR System"
rela create asset AST-003 --title "Financial Records"
rela create asset AST-004 --title "Email System"
rela create asset AST-005 --title "Source Code Repository"
```

Edit asset files to add classification. For `entities/assets/AST-001.md`:

```markdown
---
id: AST-001
title: Customer Database
owner: Data Management Team
classification: confidential
---

# Customer Database

Primary database containing customer personal information, order history, and
payment details.
```

### Identify and Document Risks

Create risks that threaten your assets:

```bash
rela create risk RSK-001 --title "Unauthorized database access"
rela create risk RSK-002 --title "Data breach via phishing"
rela create risk RSK-003 --title "Ransomware attack"
rela create risk RSK-004 --title "Insider data theft"
rela create risk RSK-005 --title "System availability loss"
```

Edit `entities/risks/RSK-001.md` to add risk details:

```markdown
---
id: RSK-001
title: Unauthorized database access
threat: External attacker
vulnerability: Weak access controls
likelihood: medium
impact: high
inherent_risk: high
residual_risk: medium
treatment_status: implementing
---

# Unauthorized Database Access

External threat actors could gain unauthorized access to the customer database
through credential theft, SQL injection, or misconfigured access controls.
```

### Link Risks to Assets

Connect risks to the assets they threaten:

```bash
rela link RSK-001 threatens AST-001  # Database access -> Customer Database
rela link RSK-002 threatens AST-001  # Phishing -> Customer Database
rela link RSK-002 threatens AST-004  # Phishing -> Email System
rela link RSK-003 threatens AST-001  # Ransomware -> Customer Database
rela link RSK-003 threatens AST-003  # Ransomware -> Financial Records
rela link RSK-003 threatens AST-005  # Ransomware -> Source Code
rela link RSK-004 threatens AST-001  # Insider threat -> Customer Database
rela link RSK-004 threatens AST-002  # Insider threat -> HR System
rela link RSK-005 threatens AST-001  # Availability -> Customer Database
```

### Verify Risk-Asset Relationships

```bash
# See what risks threaten the Customer Database
rela trace to AST-001

# See what assets RSK-003 (ransomware) threatens
rela trace from RSK-003
```

---

## Part 4: Control Implementation (IT Manager Role)

### Create Security Controls

Create controls that map to ISO 27001:2022 Annex A. Note how we use internal IDs
but store framework references as properties:

```bash
rela create control CTRL-001 --title "Information security policies"
rela create control CTRL-002 --title "Access control policy"
rela create control CTRL-003 --title "User access provisioning"
rela create control CTRL-004 --title "Malware protection"
rela create control CTRL-005 --title "Backup of information"
rela create control CTRL-006 --title "Logging and monitoring"
rela create control CTRL-007 --title "Protection of records"
rela create control CTRL-008 --title "Security awareness training"
```

Edit `entities/controls/CTRL-002.md` to add framework references:

```markdown
---
id: CTRL-002
title: Access control policy
objective: Limit access to information and systems based on business need
iso27001: "A.5.15"
iso27001_2013: "A.9.1.1"
nist_csf: "PR.AC-1"
soc2: "CC6.1"
applicability: applicable
implementation_status: implemented
---

# Access Control Policy

Access to information systems shall be controlled based on business and security
requirements through a formal access control policy.
```

### Link Risks to Treating Controls

```bash
# RSK-001 (unauthorized access) treated by access controls
rela link RSK-001 treatedBy CTRL-002
rela link RSK-001 treatedBy CTRL-003
rela link RSK-001 treatedBy CTRL-006

# RSK-002 (phishing) treated by awareness and access controls
rela link RSK-002 treatedBy CTRL-008
rela link RSK-002 treatedBy CTRL-002

# RSK-003 (ransomware) treated by malware protection and backups
rela link RSK-003 treatedBy CTRL-004
rela link RSK-003 treatedBy CTRL-005
rela link RSK-003 treatedBy CTRL-008

# RSK-004 (insider threat) treated by access controls and monitoring
rela link RSK-004 treatedBy CTRL-002
rela link RSK-004 treatedBy CTRL-003
rela link RSK-004 treatedBy CTRL-006
```

### Create Implementing Procedures

```bash
rela create procedure PROC-001 --title "User access provisioning procedure"
rela create procedure PROC-002 --title "Malware protection procedure"
rela create procedure PROC-003 --title "Backup and recovery procedure"
rela create procedure PROC-004 --title "Security monitoring procedure"
rela create procedure PROC-005 --title "Security awareness training procedure"
```

### Link Controls to Procedures and Policies

```bash
# Controls implemented by procedures
rela link CTRL-002 implementedBy PROC-001
rela link CTRL-003 implementedBy PROC-001
rela link CTRL-004 implementedBy PROC-002
rela link CTRL-005 implementedBy PROC-003
rela link CTRL-006 implementedBy PROC-004
rela link CTRL-008 implementedBy PROC-005

# Procedures mandated by policies
rela link PROC-001 mandatedBy POL-002
rela link PROC-002 mandatedBy POL-001
rela link PROC-003 mandatedBy POL-001
rela link PROC-004 mandatedBy POL-001
rela link PROC-005 mandatedBy POL-001
```

### Verify Control Traceability

```bash
# Trace from a risk to see the full treatment chain
rela trace from RSK-001

# Find the path from a risk to a specific procedure
rela trace path RSK-001 PROC-001
```

---

## Part 5: Evidence Collection (Compliance Officer Role)

### Create Evidence Records

```bash
rela create evidence EVD-001 --title "Access request tickets Q4 2024"
rela create evidence EVD-002 --title "Security awareness training completion"
rela create evidence EVD-003 --title "Antivirus deployment report"
rela create evidence EVD-004 --title "Backup verification logs"
rela create evidence EVD-005 --title "SIEM alert dashboard screenshot"
rela create evidence EVD-006 --title "User access review minutes"
```

Edit `entities/evidences/EVD-001.md` to add evidence details:

```markdown
---
id: EVD-001
title: Access request tickets Q4 2024
evidence_type: log
collected_date: 2024-12-31
valid_until: 2025-12-31
location: /evidence/access-requests/Q4-2024-export.csv
---

# Access Request Tickets Q4 2024

Export of all access request tickets from the service desk system showing the
complete access provisioning workflow.
```

### Link Evidence to Controls and Procedures

```bash
# Evidence for access controls
rela link EVD-001 evidences CTRL-002
rela link EVD-001 evidences CTRL-003
rela link EVD-001 supportsProc PROC-001
rela link EVD-006 evidences CTRL-002
rela link EVD-006 supportsProc PROC-001

# Evidence for malware protection
rela link EVD-003 evidences CTRL-004
rela link EVD-003 supportsProc PROC-002

# Evidence for backups
rela link EVD-004 evidences CTRL-005
rela link EVD-004 supportsProc PROC-003

# Evidence for monitoring
rela link EVD-005 evidences CTRL-006
rela link EVD-005 supportsProc PROC-004

# Evidence for awareness training
rela link EVD-002 evidences CTRL-008
rela link EVD-002 supportsProc PROC-005
```

### Filter Evidence by Properties

Using the `--where` flag, you can filter evidence by type, date, and other
properties:

```bash
# Find all log-type evidence
rela list evidence --where "evidence_type=log"

# Find evidence expiring before a date
rela list evidence --where "valid_until<2025-06-01"

# Find recent evidence
rela list evidence --where "collected_date>=2024-10-01"
```

---

## Part 6: Audit Preparation (Internal Auditor Role)

### Run Analysis Checks

```bash
# Find entities with no connections (potential gaps)
rela analyze orphans

# Check cardinality constraints (every risk must have treatment)
rela analyze cardinality

# Run all analysis checks
rela analyze all
```

### Find Controls Without Evidence

Using the TUI or trace commands:

```bash
# See what evidence each control has
rela show CTRL-001
rela show CTRL-007  # Check if this has evidence linked
```

### Filter Controls by Framework Reference

Find all controls in a specific ISO 27001 control family:

```bash
# All A.5.* controls (organizational controls)
rela list control --where "iso27001=A.5.*"

# All A.8.* controls (asset management)
rela list control --where "iso27001=A.8.*"

# Controls that are not yet implemented
rela list control --where "implementation_status=not_started"
```

### Verify the Complete Traceability Chain

For any given risk, you should be able to trace through to evidence:

```
Risk → Control → Procedure → Evidence
```

```bash
# Full chain from risk
rela trace from RSK-001

# Find path to specific evidence
rela trace path RSK-001 EVD-001
```

### Generate Graph Visualization

```bash
# Generate a DOT file for the full ISMS
rela graph -o isms-graph.dot

# Render to PNG (requires Graphviz)
dot -Tpng isms-graph.dot -o isms-graph.png

# Or generate SVG for web viewing
dot -Tsvg isms-graph.dot -o isms-graph.svg
```

---

## Part 7: Nonconformity Management (Quality Manager Role)

### Record Audit Findings

After an internal audit, create nonconformities:

```bash
rela create nonconformity NC-001 --title "Missing evidence for user access reviews"
rela create nonconformity NC-002 --title "Outdated backup procedure"
rela create nonconformity NC-003 --title "Training records incomplete"
```

Edit `entities/nonconformities/NC-001.md`:

```markdown
---
id: NC-001
title: Missing evidence for user access reviews
description: Quarterly user access reviews not documented for Q3 2024
identified_date: 2025-01-15
source: internal_audit
severity: major
status: open
---

# Missing Evidence for User Access Reviews

During internal audit, it was found that user access reviews for Q3 2024 were
conducted but not properly documented. The access review procedure requires
documented approval.
```

### Link Nonconformities to Affected Controls

```bash
rela link NC-001 affects CTRL-002
rela link NC-001 affects CTRL-003
rela link NC-002 affects CTRL-005
rela link NC-003 affects CTRL-008
```

### Create Corrective Actions

```bash
rela create corrective_action CA-001 --title "Document Q3 access reviews"
rela create corrective_action CA-002 --title "Update backup procedure"
rela create corrective_action CA-003 --title "Implement training tracking system"
```

Edit `entities/corrective_actions/CA-001.md`:

```markdown
---
id: CA-001
title: Document Q3 access reviews
owner: IT Security Manager
due_date: 2025-02-01
status: in_progress
---

# Document Q3 Access Reviews

Collect evidence of Q3 2024 access reviews from team leads and document in the
evidence repository.
```

### Link Corrective Actions to Nonconformities

```bash
rela link NC-001 addressedBy CA-001
rela link NC-002 addressedBy CA-002
rela link NC-003 addressedBy CA-003
```

### Track Open Nonconformities

```bash
# List open nonconformities
rela list nonconformity --where "status=open"

# List major findings
rela list nonconformity --where "severity=major"

# Sort by date
rela list nonconformity --sort identified_date
```

### Track Corrective Actions

```bash
# Find overdue corrective actions
rela list corrective_action --where "status!=completed" --where "due_date<2025-02-01"

# Find actions in progress
rela list corrective_action --where "status=in_progress"
```

---

## Part 8: Using the TUI for Daily Operations

Launch the interactive TUI:

```bash
rela tui
```

### Key TUI Features for ISMS Management

| Key               | Action                     |
| ----------------- | -------------------------- |
| `j`/`k` or arrows | Navigate                   |
| `/`               | Search across all entities |
| `Enter`           | View entity details        |
| `c`               | Create new entity          |
| `l`               | Link entities              |
| `m`               | View metamodel             |
| `a`               | Run analysis               |
| `g`               | View graph                 |
| `?`               | Help                       |

### Useful TUI Workflows

1. **Finding a control by framework reference**: Press `/`, type `A.5.15` to
   search
2. **Viewing the risk treatment chain**: Navigate to a risk, see its `treatedBy`
   relations
3. **Quick evidence review**: Navigate to a control, check `evidencedBy` count
4. **Graph exploration**: Press `g` to see visual relationships

---

## Part 9: Audit Trail with Git

Since rela stores everything as markdown files, Git provides a complete audit
trail.

### Initialize Git (if not already)

```bash
git init
git add .
git commit -m "Initial ISMS setup"
```

### View Change History

```bash
# See all changes to a specific control
git log --oneline entities/controls/CTRL-002.md

# See detailed changes
git log -p entities/controls/CTRL-002.md

# See who changed what (useful for auditors)
git blame entities/controls/CTRL-002.md
```

### Generate Audit Period Report

```bash
# All ISMS changes in the audit period
git log --since="2024-01-01" --until="2024-12-31" --oneline entities/

# Detailed change report
git log --since="2024-01-01" --format="%ai %an: %s" entities/ > audit-trail.txt
```

---

## Part 10: Multi-Framework Compliance

The metamodel supports mapping controls to multiple frameworks. This is valuable
when you need to comply with ISO 27001, SOC 2, and NIST CSF simultaneously.

### Example Multi-Framework Control

```markdown
---
id: CTRL-002
title: Access control policy
iso27001: "A.5.15"
iso27001_2013: "A.9.1.1"
nist_csf: "PR.AC-1"
soc2: "CC6.1"
---
```

### Query by Framework

```bash
# ISO 27001:2022 controls
rela list control --where "iso27001=A.*" --sort iso27001

# NIST CSF Protect category
rela list control --where "nist_csf=PR.*"

# SOC 2 Common Criteria 6 (Logical Access)
rela list control --where "soc2=CC6.*"
```

This approach allows you to:

1. Maintain one set of controls
2. Map to multiple frameworks
3. Generate framework-specific reports
4. Handle framework version changes (e.g., ISO 27001:2013 → 2022)

---

## Summary

You've built a complete ISMS with:

| Component          | Count | Purpose                       |
| ------------------ | ----- | ----------------------------- |
| Assets             | 5     | Information assets to protect |
| Risks              | 5     | Threats to assets             |
| Controls           | 8     | Security measures             |
| Policies           | 4     | Governing documents           |
| Procedures         | 5     | Implementation details        |
| Evidence           | 6     | Audit proof                   |
| Nonconformities    | 3     | Audit findings                |
| Corrective Actions | 3     | Remediation                   |

### Key Traceability Chains

```
Asset ← threatens ← Risk → treatedBy → Control → implementedBy → Procedure ← mandatedBy ← Policy
                                           ↑
                                      evidencedBy
                                           ↑
                                       Evidence
```

```
Control ← affects ← Nonconformity → addressedBy → Corrective Action
```

### Essential Commands Reference

| Task                           | Command                                         |
| ------------------------------ | ----------------------------------------------- |
| List controls by framework     | `rela list control --where "iso27001=A.9.*"`    |
| Find untreated risks           | `rela analyze cardinality`                      |
| Find controls without evidence | `rela trace to CTRL-001`                        |
| Track open findings            | `rela list nonconformity --where "status=open"` |
| View full graph                | `rela graph -o isms.dot`                        |
| Search anything                | `rela tui` then `/`                             |

---

## Next Steps

- **Expand your controls**: Add remaining Annex A controls
- **Automate evidence collection**: Link to CI/CD for automated checks
- **Set up regular reviews**: Use `review_date` filters to track policy reviews
- **Integrate with other tools**: Export to JSON/CSV for reporting dashboards

See also:

- [Export Guide](../export-guide.md) - Generate reports and integrate with
  external tools
- [Metamodel Reference](../metamodel.md) - Customize entity types and relations
- [CLI Reference](../cli-reference.md) - Complete command documentation
