---
id: TKT-3OL03W
type: ticket
title: Decide whether help/schema descriptions are config or data
kind: docs
priority: low
effort: s
status: backlog
---

## Description

`handleEntityHelp` (`GET /api/help/{entityType}`) returns an entity type's
schema and documentation based only on whether the type exists in the metamodel
— never on whether the calling principal holds any permission on it. The ACL
infrastructure is available (`attachACLRequest` opens a `readGate` for every
`/api/` request); this handler simply does not consult it.

GitHub issue #1176 (IB-review rela#1173), CONTROL-5-15. Severity: low — schema
metadata, not instance data.

## Why this is not a straightforward fix

**CLAUDE.md forbids the requested change**, in a rule marked *"settled, not
open"*: entity and property names are not confidential, and *"code must not
contort to conceal them: no filtering config endpoints per-principal."*

**And gating this endpoint alone would not close the class.** `GET
/api/v1/_schema` serves every entity type's full property set to every
principal, ungated by design — verified. A principal denied `persoon` can read
the same property names there. Gating only `/api/help/` creates the impression
of protection without the substance.

## Where the issue has a real point

The rule justifies itself with *"config lives in the repo — routinely a public
one"*. That assumption does not hold for an ISMS instance whose metamodel names
personal-data categories about real data subjects in a non-public repo.

And #1173 did widen the exposure: per-value descriptions and lifecycle prose are
business logic, not field names. The rule was written about *names* and does not
squarely address *descriptions*.

Options and a recommendation are in
`.ignored/issue-round/DISCUSS-1176-help-endpoint-gate.md`.
