---
id: TKT-8UCV32
type: ticket
title: 'Data classification overlay: labels, combination rules, subject inference, ACL audit (slice 1)'
kind: enhancement
priority: medium
effort: xl
tags: needs-design
status: done
description: 'Optional classification.yaml overlay: framework-neutral labels, levels and group rules on entity fields, with inferred subjects. CLI sync, lint, report and advisory ACL audit findings. Descriptive only. See RES-TZH38L.'
---

## Description

Let operators declare which policies and data classifications apply to entity
types, properties and relation types, without hardcoding any legal framework.
GDPR is one configuration, not a code path. rela then uses the declarations to
redact values in logs, support data-subject export and erasure, apply retention,
and lint ACL policies.

Brainstorm and options: RES-TZH38L.

## Scope

Slice 1 (RES-TZH38L § What slice 1 delivers): CLI-only static analysis from an
optional `classification.yaml`.

- In: labels with identifier roles; combination rules that produce derived
labels at `record` and `subject` scope; field assignments with the states
`needs-review` and `none`; inferred subjects and subject links with overrides;
`rela classification sync`, `lint` (also via `rela validate`) and `report`;
advisory `rela acl audit` findings with minimisation hints.
- Out: conformance profiles and `rela conformance check` (slice 2), UI badges,
ACL `@label` selectors, subject export/erase, `output` scope.
- Principles: descriptive metadata only, never blocks or changes behaviour;
not ACL; no keys in `schema.yaml`; no runtime package imports it; one concept
(labels), no levels or categories.
