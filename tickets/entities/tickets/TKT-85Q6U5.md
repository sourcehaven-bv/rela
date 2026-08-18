---
id: TKT-85Q6U5
type: ticket
title: Remove the unused doc_kind custom type from tickets/schema.yaml
kind: chore
priority: low
effort: xs
status: review
---

## Problem

`doc_kind` is defined at `tickets/schema.yaml:71` and referenced by nothing.
Dead schema config.

## How it was found

The schema-as-graph spike projected `schema.yaml` into a rela graph and ran
`analyze orphans`, which reported `doc_kind` as an orphan custom-type. Verified
independently by grep against the source, so this is a **real finding, not a
projection artifact** (most of that run's findings were artifacts — see
`.ignored/schemaspike/FINDINGS.md` §5.7).

This is the smallest possible demonstration that config-as-graph linting pays:
it is a defect class rela cannot currently detect in its own configuration.

## Scope

- Delete the `doc_kind` block from `tickets/schema.yaml`
- Confirm `rela validate` still passes

## Note

Once `analyze` over projected config lands (TKT for phase 2 of the
schema-as-graph feature), this defect class becomes detectable automatically.
