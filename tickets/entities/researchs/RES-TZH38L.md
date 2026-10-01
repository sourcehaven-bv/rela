---
id: RES-TZH38L
type: research
title: 'Framework-neutral data classification: labels, obligations, policies'
summary: 'Framework-neutral classification as descriptive metadata in an optional overlay file. One concept, labels (with identifier roles); combination rules produce derived labels; subjects inferred. Slice 1: sync, lint, report, advisory ACL audit. Slice 2: conformance profiles.'
status: done
---

# Data classification and policy applicability

## Problem

rela holds data that legal and internal policies care about: personal data,
special-category data, confidential business data. rela cannot tell which fields
those are. So it cannot help an operator see what an ACL grant exposes, report
where sensitive data sits, or check the app against the organisation's handling
rules. Hardcoding GDPR would not generalise to NIS2, HIPAA, ISO 27001 asset
classes or an organisation's own confidentiality scheme.

## Principles

- **Metadata, not enforcement.** Classification describes the data. It never
blocks, denies, redacts, or refuses a write, mail, export or AI call. Whether
sending personal data by mail is acceptable depends on context only the operator
knows. rela reports; the operator decides.
- **Layers, kept apart.**
  1. *Describe* (`classification.yaml`, per project): what the data is. Labels
and field assignments. Nothing else.
  2. *Behave* (`schema.yaml`, `acl.yaml`, mail, scripts, ...): what rela does.
Each subsystem owns its settings. Classification adds no keys to them.
  3. *Conform* (profiles, slice 2): an organisation's handling rules that tie
labels to behaviour ("`health` is not searchable"). Checked, never applied.
  4. *Check* (CLI): cross-references the layers and reports.
- **One concept: labels.** A label is an operator-chosen name for a kind of
data. The operator picks the granularity: `health` or `special-category`, or
both on one field. rela has no sensitivity scale, no categories and no framework
bundles next to labels; those would be a second mechanism for the same thing.
- **Optional, outside core.** A project without `classification.yaml` pays
nothing. The metamodel, store, API and write path never import the
classification package; only CLI tooling reads it.
- **Not ACL.** `acl.yaml` stays the only place that decides access. ACL
tooling may read classification; classification never reads or changes an ACL
decision.
- **Framework-neutral.** No framework appears in Go code. A framework is a
profile an operator writes or shares.
- **Config is not a secret.** Labels and profiles are public config,
consistent with the settled rule; only values are protected.

rela's own diagnostic output (audit `Summary`, error text, slog) should not
contain property values at all. That is a core invariant independent of
classification. The survey found it mostly holds; any gap is a separate core
ticket.

## Current state (survey, 2026-09-28)

- `PropertyDef` (`internal/metamodel/types.go:755`), `EntityDef` (:293),
`RelationDef` (:1253) and `CustomType` (:219) have no annotation field.
- Audit records (`internal/audit/audit.go:199`) store no property values; the
free-text `Summary` is the one risk.
- Long-lived value copies: version history, the search index, the mail outbox
`body`, attachments. Egress paths: Lua `print`, Lua `ai.*`, `mail.send`,
transforms, exports, MCP.
- `rela acl audit` (`internal/aclaudit`, rules A1-A9, B1-B7) reads the
metamodel through a narrow interface and works on the compiled policy.
FEAT-RCQ6SJ (effective-access map) is proposed. FEAT-OF2ZOL maps principals to
an entity type.
- Data-migration files record property renames (`internal/datamigration`).

## Model

### Labels

```yaml
labels:
  name:       { role: direct-identifier }
  contact:    { role: direct-identifier }
  birth-date: { role: quasi-identifier }
  postcode:   { role: quasi-identifier }
  gender:     { role: quasi-identifier }
  salary:     { role: attribute }
  health:     { role: attribute, description: Physical or mental health. }
```

`role` is the only attribute rela interprets. It is a different axis from the
label itself: it says *how* a kind of data identifies a person, which is what
combination rules and subject inference compute with.

| Role | Meaning |
| --- | --- |
| `direct-identifier` | Identifies on its own (name, email, national id). |
| `quasi-identifier` | Identifies in combination (birth date, postcode, gender). |
| `attribute` | Says something about a person without identifying (diagnosis, salary). |
| *(absent)* | Not about persons (a confidential contract value). |

Other keys: `description`, `reference`, and `meta` (a free-form map rela does
not interpret, available to Lua and reports).

### Combination rules produce labels

Sensitivity is often a property of a combination. ENISA's methodology for
assessing personal-data-breach severity scores "ease of identification" on what
the data allows together: a birth date alone is weak, with a name it is strong.
A combination rule matches when fields co-occur in a scope, and the matched set
then carries a **derived label**. Derived labels are ordinary labels: reports,
ACL findings and profile rules treat them the same way, so there is still one
concept.

```yaml
labels:
  identified-person:
    role: direct-identifier           # the combination identifies
    when:
      any_of:
        - all_of: ["@direct-identifier", "@quasi-identifier"]
        - count: { of: "@quasi-identifier", min: 3 }
      scope: record
  identified-health:
    role: attribute
    when:
      all_of: ["@direct-identifier", health]
      scope: subject
```

`@role` selects every label with that role; a bare name selects one label. An
operator who wants ENISA's four-step scale names derived labels after it; rela
does not impose one.

### Scopes

| Scope | Fields co-occur when… | Evaluated |
| --- | --- | --- |
| `record` | they are on the same entity or relation | load time, schema only |
| `subject` | they are reachable from one subject via subject links | load time, schema only |
| `output` | they appear in one response, view, export, log line, prompt | later |

### Subject inference

Subjects and links are derived, not declared.

- **Subject type.** A type whose own fields carry a `direct-identifier` role,
directly or through a derived label at record scope.
- **Subject link.** Every relation type with a subject type at one end.
Direction does not matter for analysis: whoever sees both ends and the edge can
join them. Reach is one hop by default; hubs (`person -> team -> project`) would
otherwise pull in most of the graph.
- **Principals.** The type principals resolve to (FEAT-OF2ZOL) is a subject.
rela's attribution data (audit principal, `last_edited_by_user`) is personal
data about users and appears as a built-in link.
- **Provenance.** The report says why ("subject: `person.email` is a direct
identifier").
- **Overrides** in `classification.yaml`: `subject: {company: false}`,
`subject_link: {watches: false}`, `subject_hops: {of: 2}`.

Inference errs toward inclusion, which is safe for analysis. A future erase
command needs explicit per-link actions (`delete`, `anonymise`, `detach`,
`retain`); that belongs to the erasure ticket.

## Field assignments (overlay)

Assignments live in `classification.yaml`, not in `schema.yaml`. The schema does
not grow, and the concern stays out of core rela, which many apps do not need.

```yaml
assign:
  person:
    title:      [name]
    body:       [health]
    email:      [contact]
    nickname:   none
  employment:
    birth_date: [birth-date]
    postcode:   [postcode]
    gender:     [gender]
    salary:     [salary]
    start_date: needs-review
  sick-leave:
    diagnosis:  [health]
assign_relations: {}          # relation properties, same shape
```

Each field has one of three states, spelled out by name:

- a list of labels: reviewed, carries these labels;
- `none`: reviewed, carries no label;
- `needs-review`: not reviewed yet.

A missing key is an error. `none` and `needs-review` are reserved and cannot be
label names. Name patterns (`"*.email"`) are left out: they would count as a
review of fields nobody looked at.

### Generate and lint

- **`rela classification sync`** adds every type, property, `title` and `body`
without an entry as `needs-review`, carries labels across renames recorded in
data-migration files, and reports entries whose field no longer exists. It never
deletes a label on its own. It may suggest labels from same-named fields on
other types.
- **`rela classification lint`** (and `rela validate` when the file exists)
errors on a missing entry, `needs-review`, an entry for a field that does not
exist, an undefined label, an invalid combination rule, or an unknown key.

## Conformance profiles (slice 2)

A profile holds an organisation's handling rules. Rules reference labels
directly; there are no categories or levels in between. A framework such as GDPR
is a profile.

```yaml
# profiles/org-privacy.yaml (org-wide, shareable across projects)
profile: org-privacy
reference: https://intranet/privacy-handling-standard
rules:
  special-not-searchable:
    when:   { label: [health, ethnicity, religion, biometric] }
    expect: { searchable: false }
    severity: error
  identified-not-to-clients:
    when:   { label: identified-person }
    expect: { readable_by: { client_ceilings: none } }
    severity: error
  no-health-in-mail:
    when:   { label: health }
    expect: { in_mail_templates: false }
    severity: warning
```

```yaml
# classification.yaml (project)
profiles: [org-privacy]
waivers:
  - rule: no-health-in-mail
    field: sick-leave.diagnosis
    reason: Occupational health service receives it by contract (DPA-2025-14).
```

- **Check, not drive.** `rela conformance check` evaluates the rules and fails
on unwaived `error` findings; the operator decides whether CI runs it. A profile
never changes behaviour. `--fix` may write a required setting into core config
for review.
- **Facts.** Rules compare against a fixed fact vocabulary that each
subsystem exposes about its own config: `searchable` (if core gains it),
`readable_by` (compiled ACL), `in_views`/`in_exports`/`in_feeds`,
`in_mail_templates`, `versioned`, and a heuristic `in_lua_scripts`. A fact rela
cannot establish is `unknown`, never `pass`.
- **Waivers** need a reason and live in the project. A stale waiver is a lint
error.
- **Shared vocabulary.** A profile shared across projects assumes shared label
names. A rule that references a label the project does not define is a warning
("rule can never match"), which catches vocabulary drift.

## Consumers

| Consumer | What it does | Slice |
| --- | --- | --- |
| `rela classification sync` | Generate/update assignments; carry labels across migration renames; report stale entries. | 1 |
| `rela classification lint` / `rela validate` | Every field reviewed; no stale entries; no unknown labels or keys. | 1 |
| `rela classification report` | Labels per type, derived labels per type and per subject profile, inferred subjects and links with provenance. | 1 |
| `rela acl audit` findings | Which labels (own and derived) each role and client ceiling can read, at record and subject scope, with minimisation hints. Advisory. | 1 |
| `rela conformance check` | Profiles, facts, waivers. | 2 |
| Data-entry UI badges | Labels on fields; served only when the file exists. | later |
| ACL `@label` selectors | ACL-side feature reusing the vocabulary. | later |
| Subject export / erase | Own config for per-link actions; uses inferred subjects. | later |
| `output` scope | Rate ad-hoc outputs. | later |

## What slice 1 delivers

CLI-only: `sync`, `lint`, `report`, and the `rela acl audit` findings. No
runtime component reads classification.

Worked example: `person` (name, email), `employment` (birth date, postcode,
gender, salary), `sick-leave` (diagnosis), both linked to `person` by `of`.

Record scope:

1. `employment` carries derived label `identified-person` without a name
(three quasi-identifiers), so it is also inferred as a subject type.
2. Per role: "`analyst` can read `employment{birth_date, postcode, gender}`:
derived label `identified-person`. Drop `postcode` or `gender` to break it." The
same for MCP/AI client ceilings.

Subject scope adds:

1. `sick-leave.diagnosis` combined with `person.title` carries
`identified-health`.
2. Join-aware findings: a role that can read `sick-leave.diagnosis`, the `of`
relation and `person.title` can combine them; the finding names the grants whose
removal breaks the combination.
3. Inventory: which types and fields can hold data about a person.

## Design tensions

- **Static vs conditional.** A field may be personal for some rows only. v1 is
static.
- **Worst case.** Static analysis assumes every field may be filled.
- **Bodies.** Free text; only a type-level `body` assignment covers it.
- **IDs as data.** A slug id such as `jan-jansen` is personal data and stays in
audit and version rows. The report flags subject types with non-opaque ids.
- **Advice, not compliance.** rela reports; it cannot certify. Output says so.
- **Breach scoring.** ENISA's full score also weighs processing context and
breach circumstances. Those describe an incident; out of scope.

## Decisions

1. Overlay file, not schema keys; `sync` and `lint` mitigate drift.
2. No behaviour attributes; behaviour stays in core config.
3. Field states `needs-review` and `none`.
4. Conformance profiles are a separate layer, in slice 2.
5. Slice 1 answers the ACL question only.
6. One concept: labels. No levels, categories or framework bundles;
combination rules produce derived labels.
